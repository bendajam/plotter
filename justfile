# Garden Plotter — task runner
# Install just: brew install just

port := "8080"
bin  := "./plotter"

# List available recipes
default:
    @just --list

# Build the binary
build:
    CGO_ENABLED=0 go build -o {{bin}} .

# Run directly with go run (no build step)
run:
    go run .

# Build then start the binary
start: build
    {{bin}}

# Kill any process on the port, rebuild, and start
restart: stop build
    {{bin}}

# Stop whatever is listening on the port
stop:
    @lsof -ti :{{port}} | xargs kill -9 2>/dev/null && echo "Stopped process on :{{port}}" || echo "Nothing running on :{{port}}"

# Watch for file changes and auto-restart (requires air: go install github.com/air-verse/air@latest)
dev:
    air

# Run tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Tidy go modules
tidy:
    go mod tidy

# Remove the built binary
clean:
    rm -f {{bin}}

# Wipe the database (irreversible!)
drop-db:
    @echo "This will delete all data in plotter.db. Press Ctrl-C to cancel, Enter to continue."
    @read _confirm
    rm -f plotter.db
    @echo "Database deleted."

# Open the app in the browser
open:
    open http://localhost:{{port}}

# Tail the running app's output (attach to background process stdout)
logs:
    @lsof -ti :{{port}} | xargs -I{} sh -c 'echo "PID: {}"; cat /proc/{}/fd/1 2>/dev/null || echo "Cannot attach to logs — run foreground with: just run"'

# Show what is listening on the port
status:
    @lsof -i :{{port}} || echo "Nothing on :{{port}}"

# ── Deployment ────────────────────────────────────────────────

deploy_dir := "/opt/plotter"

# Install binary + assets to deploy_dir (run as root or with sudo)
install: build
    install -d {{deploy_dir}}/data/uploads/plots {{deploy_dir}}/data/uploads/markers {{deploy_dir}}/data/backups
    install -d {{deploy_dir}}/deploy
    install -m 755 ./plotter         {{deploy_dir}}/plotter
    cp -r static templates           {{deploy_dir}}/
    install -m 755 deploy/backup.py  {{deploy_dir}}/deploy/backup.py
    install -m 755 deploy/restore.py {{deploy_dir}}/deploy/restore.py
    install -m 644 deploy/gcs.py     {{deploy_dir}}/deploy/gcs.py
    @echo "Installed to {{deploy_dir}}"

# Install and enable the systemd service (run as root)
install-service: install
    groupadd plotter 2>/dev/null || true
    useradd --system --no-create-home --shell /usr/sbin/nologin --gid plotter plotter 2>/dev/null || true
    chown -R plotter:plotter {{deploy_dir}}/data
    install -m 644 deploy/plotter.service /etc/systemd/system/plotter.service
    systemctl daemon-reload
    systemctl enable plotter
    systemctl restart plotter
    systemctl status plotter

# Reload after a binary update (run as root)
deploy: build
    install -m 755 ./plotter {{deploy_dir}}/plotter
    cp -r static templates   {{deploy_dir}}/
    systemctl restart plotter
    @echo "Deployed and restarted."

# Push local DB and uploads to GCS using your local GOOGLE_APPLICATION_CREDENTIALS
# Requires GCS_BUCKET and GOOGLE_APPLICATION_CREDENTIALS to be set in your environment
push:
    PYTHONPATH=deploy \
    PLOTTER_DB=plotter.db \
    PLOTTER_UPLOAD_DIR=uploads \
    PLOTTER_BACKUP_DIR=tmp/backups \
    python3 deploy/backup.py

# Run a manual full backup
backup:
    PLOTTER_DB={{deploy_dir}}/data/plotter.db \
    PLOTTER_UPLOAD_DIR={{deploy_dir}}/data/uploads \
    PLOTTER_BACKUP_DIR={{deploy_dir}}/data/backups \
    python3 {{deploy_dir}}/deploy/backup.py

# Gzip the uploads/images directory only
backup-uploads:
    PLOTTER_UPLOAD_DIR={{deploy_dir}}/data/uploads \
    PLOTTER_BACKUP_DIR={{deploy_dir}}/data/backups \
    python3 {{deploy_dir}}/deploy/backup.py --uploads-only

# List available backups in GCS
restore-list:
    python3 {{deploy_dir}}/deploy/restore.py --list

# Restore latest backup from GCS (sudo required to manage the service)
# To restore a specific backup: just restore 20260413_025047
restore timestamp='':
    python3 {{deploy_dir}}/deploy/restore.py {{timestamp}}
