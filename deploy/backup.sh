#!/usr/bin/env bash
# Backs up the Plotter SQLite database and uploads directory.
# Designed to be run as a cron job, e.g.:
#
#   0 3 * * * /opt/plotter/deploy/backup.sh >> /var/log/plotter-backup.log 2>&1
#
# Keeps the last KEEP_DAYS days of backups and deletes older ones.

set -euo pipefail

DB_PATH="${PLOTTER_DB:-/opt/plotter/data/plotter.db}"
UPLOAD_DIR="${PLOTTER_UPLOAD_DIR:-/opt/plotter/data/uploads}"
BACKUP_DIR="${PLOTTER_BACKUP_DIR:-/opt/plotter/backups}"
KEEP_DAYS="${PLOTTER_BACKUP_KEEP_DAYS:-30}"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
mkdir -p "$BACKUP_DIR"

# SQLite online backup — safe to run while the server is live.
sqlite3 "$DB_PATH" ".backup '$BACKUP_DIR/plotter_${TIMESTAMP}.db'"
echo "[$(date -Iseconds)] DB backup → $BACKUP_DIR/plotter_${TIMESTAMP}.db"

# Snapshot uploads directory.
tar -czf "$BACKUP_DIR/uploads_${TIMESTAMP}.tar.gz" -C "$(dirname "$UPLOAD_DIR")" "$(basename "$UPLOAD_DIR")"
echo "[$(date -Iseconds)] Uploads backup → $BACKUP_DIR/uploads_${TIMESTAMP}.tar.gz"

# Prune backups older than KEEP_DAYS.
find "$BACKUP_DIR" -name "plotter_*.db"        -mtime +"$KEEP_DAYS" -delete
find "$BACKUP_DIR" -name "uploads_*.tar.gz"    -mtime +"$KEEP_DAYS" -delete
echo "[$(date -Iseconds)] Pruned backups older than ${KEEP_DAYS} days."
