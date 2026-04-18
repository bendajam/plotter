#!/usr/bin/env python3
"""
Plotter backup script — backs up the SQLite database and uploads directory.

Designed to be run as a cron job, e.g.:
    0 3 * * * /opt/plotter/deploy/backup.py >> /var/log/plotter-backup.log 2>&1

Keeps the last KEEP_DAYS days of local backups and deletes older ones.
If GOOGLE_APPLICATION_CREDENTIALS and GCS_BUCKET are set, backups are
uploaded to Google Cloud Storage using the GCS XML API directly (no
gcloud or gsutil required — only the Python standard library).
"""

import base64
import glob
import json
import os
import sqlite3
import subprocess
import sys
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

# ── Config from environment ───────────────────────────────────

DB_PATH        = Path(os.environ.get("PLOTTER_DB",          "/opt/plotter/data/plotter.db"))
UPLOAD_DIR     = Path(os.environ.get("PLOTTER_UPLOAD_DIR",  "/opt/plotter/data/uploads"))
BACKUP_DIR     = Path(os.environ.get("PLOTTER_BACKUP_DIR",  "/opt/plotter/data/backups"))
KEEP_DAYS      = int(os.environ.get("PLOTTER_BACKUP_KEEP_DAYS", "30"))
GCS_BUCKET     = os.environ.get("GCS_BUCKET", "")
CREDENTIALS    = os.environ.get("GOOGLE_APPLICATION_CREDENTIALS", "")

TIMESTAMP = datetime.now().strftime("%Y%m%d_%H%M%S")


def log(msg: str) -> None:
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S+00:00")
    print(f"[{ts}] {msg}", flush=True)


# ── GCS auth ──────────────────────────────────────────────────

def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def gcs_token(credentials_path: str) -> str:
    """
    Read a service account JSON key, build a signed JWT, and exchange it
    for a short-lived GCS OAuth2 access token.
    Uses only the Python standard library + openssl (via subprocess).
    """
    with open(credentials_path) as f:
        sa = json.load(f)

    client_email = sa["client_email"]
    private_key  = sa["private_key"]

    now = int(time.time())
    header = _b64url(json.dumps({"alg": "RS256", "typ": "JWT"}).encode())
    claims = _b64url(json.dumps({
        "iss":   client_email,
        "scope": "https://www.googleapis.com/auth/devstorage.read_write",
        "aud":   "https://oauth2.googleapis.com/token",
        "iat":   now,
        "exp":   now + 3600,
    }).encode())

    signing_input = f"{header}.{claims}".encode()

    # Sign with the private key using openssl (avoids needing cryptography lib).
    # Write the key to a temp file to avoid shell quoting issues with the
    # multi-line PEM block.
    import tempfile
    with tempfile.NamedTemporaryFile(delete=True, suffix=".pem") as kf:
        kf.write(private_key.encode())
        kf.flush()
        result = subprocess.run(
            ["openssl", "dgst", "-sha256", "-sign", kf.name],
            input=signing_input,
            capture_output=True,
        )
    if result.returncode != 0:
        raise RuntimeError(f"openssl signing failed: {result.stderr.decode()}")

    signature = _b64url(result.stdout)
    jwt = f"{header}.{claims}.{signature}"

    # Exchange JWT for access token
    data = urllib.parse.urlencode({
        "grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
        "assertion":  jwt,
    }).encode()
    req = urllib.request.Request(
        "https://oauth2.googleapis.com/token",
        data=data,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())["access_token"]


def gcs_upload(token: str, local_path: Path, bucket: str, object_path: str) -> None:
    """Upload a file to GCS using the XML API (plain HTTP PUT)."""
    url = f"https://storage.googleapis.com/{bucket}/{object_path}"
    with open(local_path, "rb") as f:
        data = f.read()
    req = urllib.request.Request(
        url,
        data=data,
        method="PUT",
        headers={
            "Authorization":  f"Bearer {token}",
            "Content-Type":   "application/octet-stream",
            "Content-Length": str(len(data)),
        },
    )
    with urllib.request.urlopen(req) as resp:
        if resp.status not in (200, 204):
            raise RuntimeError(f"GCS upload failed: HTTP {resp.status}")


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


def prune_old_backups() -> None:
    """Delete local backups older than KEEP_DAYS days."""
    cutoff = time.time() - KEEP_DAYS * 86400
    patterns = ["plotter_*.db", "uploads_*.tar.gz"]
    removed = 0
    for pattern in patterns:
        for path in glob.glob(str(BACKUP_DIR / pattern)):
            if os.path.getmtime(path) < cutoff:
                os.remove(path)
                removed += 1
    log(f"Pruned {removed} local backup(s) older than {KEEP_DAYS} days.")


# ── Main ──────────────────────────────────────────────────────

def main() -> None:
    BACKUP_DIR.mkdir(parents=True, exist_ok=True)

    # DB backup
    db_backup = BACKUP_DIR / f"plotter_{TIMESTAMP}.db"
    backup_db(db_backup)
    log(f"DB backup → {db_backup}")

    # Uploads backup
    uploads_backup = BACKUP_DIR / f"uploads_{TIMESTAMP}.tar.gz"
    backup_uploads(uploads_backup)
    log(f"Uploads backup → {uploads_backup}")

    # GCS upload
    if CREDENTIALS and GCS_BUCKET:
        log("Fetching GCS access token...")
        try:
            token = gcs_token(CREDENTIALS)

            gcs_upload(token, db_backup, GCS_BUCKET, f"backups/plotter_{TIMESTAMP}.db")
            log(f"Uploaded DB → gs://{GCS_BUCKET}/backups/plotter_{TIMESTAMP}.db")

            gcs_upload(token, uploads_backup, GCS_BUCKET, f"backups/uploads_{TIMESTAMP}.tar.gz")
            log(f"Uploaded uploads → gs://{GCS_BUCKET}/backups/uploads_{TIMESTAMP}.tar.gz")

        except Exception as e:
            log(f"ERROR: GCS upload failed: {e}")
            sys.exit(1)
    elif not CREDENTIALS:
        log("Skipping GCS upload: GOOGLE_APPLICATION_CREDENTIALS not set.")
    else:
        log("Skipping GCS upload: GCS_BUCKET not set.")

    prune_old_backups()


if __name__ == "__main__":
    main()
