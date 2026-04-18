#!/usr/bin/env python3
"""
Plotter restore script — downloads and restores a backup from GCS.

Usage:
    restore.py                   # restore the latest backup
    restore.py 20260413_025047   # restore a specific timestamp
    restore.py --list            # list available backups in GCS

Requires:
    GOOGLE_APPLICATION_CREDENTIALS — path to service account JSON key
    GCS_BUCKET                     — GCS bucket name

The service account needs roles/storage.objectAdmin (or at minimum
roles/storage.objectViewer) on the bucket to list and download backups.

The script stops the plotter service before restoring and restarts it
after. Run with sudo (or as root) so it can manage the service.
"""

import argparse
import os
import shutil
import subprocess
import sys
import tarfile
import tempfile
from datetime import datetime, timezone
from pathlib import Path

import gcs

# ── Config from environment ───────────────────────────────────

DB_PATH     = Path(os.environ.get("PLOTTER_DB",         "/opt/plotter/data/plotter.db"))
UPLOAD_DIR  = Path(os.environ.get("PLOTTER_UPLOAD_DIR", "/opt/plotter/data/uploads"))
GCS_BUCKET  = os.environ.get("GCS_BUCKET", "")
CREDENTIALS = os.environ.get("GOOGLE_APPLICATION_CREDENTIALS", "")


def log(msg: str) -> None:
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S+00:00")
    print(f"[{ts}] {msg}", flush=True)


# ── GCS helpers ───────────────────────────────────────────────

def list_backups(token: str) -> list[str]:
    """Return sorted list of backup timestamps available in GCS."""
    objects = gcs.list_objects(token, GCS_BUCKET, "backups/plotter_")
    # Object names look like: backups/plotter_20260413_025047.db
    timestamps = []
    for name in objects:
        stem = Path(name).stem  # plotter_20260413_025047
        ts = stem.removeprefix("plotter_")
        if ts:
            timestamps.append(ts)
    return sorted(timestamps)


def latest_timestamp(token: str) -> str:
    """Return the most recent backup timestamp from GCS."""
    timestamps = list_backups(token)
    if not timestamps:
        raise RuntimeError("No backups found in GCS.")
    return timestamps[-1]


# ── Service control ───────────────────────────────────────────

def service_running() -> bool:
    result = subprocess.run(
        ["systemctl", "is-active", "--quiet", "plotter"],
        capture_output=True,
    )
    return result.returncode == 0


def stop_service() -> bool:
    """Stop plotter service. Returns True if it was running."""
    was_running = service_running()
    if was_running:
        log("Stopping plotter service...")
        subprocess.run(["systemctl", "stop", "plotter"], check=True)
    return was_running


def start_service() -> None:
    log("Starting plotter service...")
    subprocess.run(["systemctl", "start", "plotter"], check=True)


# ── Restore ───────────────────────────────────────────────────

def restore_db(token: str, timestamp: str) -> None:
    object_path = f"backups/plotter_{timestamp}.db"
    log(f"Downloading gs://{GCS_BUCKET}/{object_path} ...")

    with tempfile.NamedTemporaryFile(delete=False, suffix=".db") as tmp:
        tmp_path = Path(tmp.name)

    try:
        gcs.download(token, GCS_BUCKET, object_path, tmp_path)
        DB_PATH.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(str(tmp_path), str(DB_PATH))
        log(f"DB restored → {DB_PATH}")
    except Exception:
        tmp_path.unlink(missing_ok=True)
        raise


def restore_uploads(token: str, timestamp: str) -> None:
    object_path = f"backups/uploads_{timestamp}.tar.gz"
    log(f"Downloading gs://{GCS_BUCKET}/{object_path} ...")

    with tempfile.NamedTemporaryFile(delete=False, suffix=".tar.gz") as tmp:
        tmp_path = Path(tmp.name)

    try:
        gcs.download(token, GCS_BUCKET, object_path, tmp_path)

        # Extract into the parent of UPLOAD_DIR so uploads/ is restored in place
        parent = UPLOAD_DIR.parent
        parent.mkdir(parents=True, exist_ok=True)

        log(f"Extracting uploads → {parent} ...")
        with tarfile.open(tmp_path, "r:gz") as tar:
            tar.extractall(path=parent)

        log(f"Uploads restored → {UPLOAD_DIR}")
    finally:
        tmp_path.unlink(missing_ok=True)


# ── Main ──────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(description="Plotter restore utility")
    parser.add_argument("timestamp", nargs="?",
                        help="Backup timestamp to restore (e.g. 20260413_025047). "
                             "Defaults to the latest available backup.")
    parser.add_argument("--list", action="store_true",
                        help="List available backups in GCS and exit.")
    parser.add_argument("--db-only", action="store_true",
                        help="Restore database only; leave uploads untouched.")
    parser.add_argument("--uploads-only", action="store_true",
                        help="Restore uploads only; leave database untouched.")
    args = parser.parse_args()

    if not CREDENTIALS:
        print("ERROR: GOOGLE_APPLICATION_CREDENTIALS is not set.", file=sys.stderr)
        sys.exit(1)
    if not GCS_BUCKET:
        print("ERROR: GCS_BUCKET is not set.", file=sys.stderr)
        sys.exit(1)

    log("Fetching GCS access token...")
    token = gcs.get_token(CREDENTIALS)

    # --list: just print available backups and exit
    if args.list:
        timestamps = list_backups(token)
        if not timestamps:
            print("No backups found.")
        else:
            print(f"{'Timestamp':<20}  GCS objects")
            print("-" * 60)
            for ts in reversed(timestamps):
                print(f"  {ts}  →  plotter_{ts}.db  |  uploads_{ts}.tar.gz")
        return

    # Resolve timestamp
    timestamp = args.timestamp or latest_timestamp(token)
    log(f"Restoring backup: {timestamp}")

    was_running = stop_service()
    try:
        if not args.uploads_only:
            restore_db(token, timestamp)
        if not args.db_only:
            restore_uploads(token, timestamp)
    except Exception as e:
        log(f"ERROR: restore failed: {e}")
        if was_running:
            log("Attempting to restart service despite error...")
            start_service()
        sys.exit(1)

    if was_running:
        start_service()

    log("Restore complete.")


if __name__ == "__main__":
    main()
