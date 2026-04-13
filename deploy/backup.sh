#!/usr/bin/env bash
# Backs up the Plotter SQLite database and uploads directory.
# Designed to be run as a cron job, e.g.:
#
#   0 3 * * * /opt/plotter/deploy/backup.sh >> /var/log/plotter-backup.log 2>&1
#
# Keeps the last KEEP_DAYS days of local backups and deletes older ones.
# If GOOGLE_APPLICATION_CREDENTIALS and GCS_BUCKET are set, backups are
# also uploaded to Google Cloud Storage.

set -euo pipefail

DB_PATH="${PLOTTER_DB:-/opt/plotter/data/plotter.db}"
UPLOAD_DIR="${PLOTTER_UPLOAD_DIR:-/opt/plotter/data/uploads}"
BACKUP_DIR="${PLOTTER_BACKUP_DIR:-/opt/plotter/data/backups}"
KEEP_DAYS="${PLOTTER_BACKUP_KEEP_DAYS:-30}"
GCS_BUCKET="${GCS_BUCKET:-}"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
mkdir -p "$BACKUP_DIR"

# ── Local backup ──────────────────────────────────────────────

# SQLite online backup — safe to run while the server is live.
DB_BACKUP="$BACKUP_DIR/plotter_${TIMESTAMP}.db"
sqlite3 "$DB_PATH" ".backup '$DB_BACKUP'"
echo "[$(date -Iseconds)] DB backup → $DB_BACKUP"

# Snapshot uploads directory.
UPLOADS_BACKUP="$BACKUP_DIR/uploads_${TIMESTAMP}.tar.gz"
tar -czf "$UPLOADS_BACKUP" -C "$(dirname "$UPLOAD_DIR")" "$(basename "$UPLOAD_DIR")"
echo "[$(date -Iseconds)] Uploads backup → $UPLOADS_BACKUP"

# ── GCS upload ────────────────────────────────────────────────

if [[ -n "${GOOGLE_APPLICATION_CREDENTIALS:-}" && -n "$GCS_BUCKET" ]]; then
    export GOOGLE_APPLICATION_CREDENTIALS
    GCS_PATH="gs://${GCS_BUCKET}/backups"

    gsutil -q cp "$DB_BACKUP"       "$GCS_PATH/plotter_${TIMESTAMP}.db"
    echo "[$(date -Iseconds)] Uploaded DB → $GCS_PATH/plotter_${TIMESTAMP}.db"

    gsutil -q cp "$UPLOADS_BACKUP"  "$GCS_PATH/uploads_${TIMESTAMP}.tar.gz"
    echo "[$(date -Iseconds)] Uploaded uploads → $GCS_PATH/uploads_${TIMESTAMP}.tar.gz"
else
    if [[ -z "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]]; then
        echo "[$(date -Iseconds)] Skipping GCS upload: GOOGLE_APPLICATION_CREDENTIALS not set."
    else
        echo "[$(date -Iseconds)] Skipping GCS upload: GCS_BUCKET not set."
    fi
fi

# ── Prune old local backups ───────────────────────────────────

find "$BACKUP_DIR" -name "plotter_*.db"     -mtime +"$KEEP_DAYS" -delete
find "$BACKUP_DIR" -name "uploads_*.tar.gz" -mtime +"$KEEP_DAYS" -delete
echo "[$(date -Iseconds)] Pruned local backups older than ${KEEP_DAYS} days."
