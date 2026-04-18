"""
Shared GCS helpers for backup.py and restore.py.
Uses only the Python standard library + openssl — no gcloud or gsutil needed.

The service account must have roles/storage.objectAdmin on the bucket
(objectCreator is enough for upload-only; objectViewer is needed for
listing and downloading during restore).
"""

import base64
import json
import os
import platform
import ssl
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request
from pathlib import Path


def _ssl_context() -> ssl.SSLContext:
    """
    Return an SSL context with valid CA certificates.
    Python installed from python.org on macOS does not bundle CA certs,
    causing CERTIFICATE_VERIFY_FAILED errors. This tries certifi first,
    then the macOS system bundle, then falls back to the default (works on Linux).
    """
    try:
        import certifi
        return ssl.create_default_context(cafile=certifi.where())
    except ImportError:
        pass

    if platform.system() == "Darwin":
        system_ca = "/etc/ssl/cert.pem"
        if os.path.exists(system_ca):
            return ssl.create_default_context(cafile=system_ca)

    return ssl.create_default_context()


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def get_token(credentials_path: str) -> str:
    """
    Read a service account JSON key, build a signed JWT, and exchange it
    for a short-lived OAuth2 access token with read/write storage scope.
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

    jwt = f"{header}.{claims}.{_b64url(result.stdout)}"

    data = urllib.parse.urlencode({
        "grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
        "assertion":  jwt,
    }).encode()
    req = urllib.request.Request(
        "https://oauth2.googleapis.com/token",
        data=data,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
    )
    with urllib.request.urlopen(req, context=_ssl_context()) as resp:
        return json.loads(resp.read())["access_token"]


def upload(token: str, local_path: Path, bucket: str, object_path: str) -> None:
    """Upload a local file to GCS using the XML API (HTTP PUT)."""
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
    with urllib.request.urlopen(req, context=_ssl_context()) as resp:
        if resp.status not in (200, 204):
            raise RuntimeError(f"GCS upload failed: HTTP {resp.status}")


def download(token: str, bucket: str, object_path: str, dest: Path) -> None:
    """Download a GCS object to a local file using the XML API (HTTP GET)."""
    url = f"https://storage.googleapis.com/{bucket}/{urllib.parse.quote(object_path, safe='/')}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    with urllib.request.urlopen(req, context=_ssl_context()) as resp:
        dest.write_bytes(resp.read())


def list_objects(token: str, bucket: str, prefix: str) -> list[str]:
    """
    Return a sorted list of object names in a GCS bucket matching a prefix.
    Uses the GCS JSON API.
    """
    url = (
        f"https://storage.googleapis.com/storage/v1/b/{bucket}/o"
        f"?prefix={urllib.parse.quote(prefix)}"
    )
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    with urllib.request.urlopen(req, context=_ssl_context()) as resp:
        data = json.loads(resp.read())
    return sorted(item["name"] for item in data.get("items", []))
