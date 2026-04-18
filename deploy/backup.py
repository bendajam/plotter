#!/usr/bin/env python3
"""
Plotter backup script — backs up the SQLite database and uploads directory.

Usage:
    backup.py                # full backup: DB + uploads
    backup.py --uploads-only # gzip uploads only (skips DB)

Designed to be run as a cron job, e.g.:
    0 3 * * * python3 /opt/plotter/deploy/backup.py >> /var/log/plotter-backup.log 2>&1

Keeps the last KEEP_DAYS days of local backups and deletes older ones.
If GOOGLE_APPLICATION_CREDENTIALS and GCS_BUCKET are set, backups are
uploaded to Google Cloud Storage (no gcloud or gsutil required).
"""

import argparse
import glob
import os
import sqlite3
import sys
import tarfile
import time
from datetime import datetime, timezone
from pathlib import Path

import gcs

# ── Config from environment ───────────────────────────────────

DB_PATH     = Path(os.environ.get("PLOTTER_DB",               "/opt/plotter/data/plotter.db"))
UPLOAD_DIR  = Path(os.environ.get("PLOTTER_UPLOAD_DIR",       "/opt/plotter/data/uploads"))
BACKUP_DIR  = Path(os.environ.get("PLOTTER_BACKUP_DIR",       "/opt/plotter/data/backups"))
KEEP_DAYS   = int(os.environ.get("PLOTTER_BACKUP_KEEP_DAYS",  "30"))
GCS_BUCKET  = os.environ.get("GCS_BUCKET", "")
CREDENTIALS = os.environ.get("GOOGLE_APPLICATION_CREDENTIALS", "")

TIMESTAMP = datetime.now().strftime("%Y%m%d_%H%M%S")


def log(msg: str) -> None:
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S+00:00")
    print(f"[{ts}] {msg}", flush=True)


# ── Local backup ──────────────────────────────────────────────

def backup_db(dest: Path) -> None:
    """SQLite online backup — safe to run while the server is live."""
    src = sqlite3.connect(str(DB_PATH))
    dst = sqlite3.connect(str(dest))
    src.backup(dst)
    src.close()
    dst.close()


def backup_uploads(dest: Path) -> None:
    """Snapshot the uploads directory into a tar.gz archive."""
    with tarfile.open(dest, "w:gz") as tar:
        tar.add(UPLOAD_DIR, arcname=UPLOAD_DIR.name)


def prune_old_backups(uploads_only: bool) -> None:
    """Delete local backups older than KEEP_DAYS days."""
    cutoff = time.time() - KEEP_DAYS * 86400
    patterns = ["uploads_*.tar.gz"] if uploads_only else ["plotter_*.db", "uploads_*.tar.gz"]
    removed = 0
    for pattern in patterns:
        for path in glob.glob(str(BACKUP_DIR / pattern)):
            if os.path.getmtime(path) < cutoff:
                os.remove(path)
                removed += 1
    log(f"Pruned {removed} local backup(s) older than {KEEP_DAYS} days.")


# ── Main ──────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(description="Plotter backup utility")
    parser.add_argument("--uploads-only", action="store_true",
                        help="Gzip uploads directory only; skip the database backup")
    args = parser.parse_args()

    BACKUP_DIR.mkdir(parents=True, exist_ok=True)

    db_backup      = None
    uploads_backup = BACKUP_DIR / f"uploads_{TIMESTAMP}.tar.gz"

    # DB backup
    if not args.uploads_only:
        db_backup = BACKUP_DIR / f"plotter_{TIMESTAMP}.db"
        backup_db(db_backup)
        log(f"DB backup → {db_backup}")

    # Uploads backup
    backup_uploads(uploads_backup)
    log(f"Uploads backup → {uploads_backup}")

    # GCS upload
    if CREDENTIALS and GCS_BUCKET:
        log("Fetching GCS access token...")
        try:
            token = gcs.get_token(CREDENTIALS)

            if db_backup:
                gcs.upload(token, db_backup, GCS_BUCKET, f"backups/plotter_{TIMESTAMP}.db")
                log(f"Uploaded DB → gs://{GCS_BUCKET}/backups/plotter_{TIMESTAMP}.db")

            gcs.upload(token, uploads_backup, GCS_BUCKET, f"backups/uploads_{TIMESTAMP}.tar.gz")
            log(f"Uploaded uploads → gs://{GCS_BUCKET}/backups/uploads_{TIMESTAMP}.tar.gz")

        except Exception as e:
            log(f"ERROR: GCS upload failed: {e}")
            sys.exit(1)
    elif not CREDENTIALS:
        log("Skipping GCS upload: GOOGLE_APPLICATION_CREDENTIALS not set.")
    else:
        log("Skipping GCS upload: GCS_BUCKET not set.")

    prune_old_backups(args.uploads_only)


if __name__ == "__main__":
    main()
