# Plotter

A personal garden history tracker. Drop markers on an image of your plot, log notes and harvests, track plant taxonomy, and build up a living record of your garden over time.

## Stack

- **Backend**: Go 1.22, [chi](https://github.com/go-chi/chi) router
- **Database**: SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, no CGO required)
- **Frontend**: [HTMX 2](https://htmx.org/) for server-driven partials, vanilla JS canvas for the map
- **Templates**: Go `html/template`
- **Task runner**: [just](https://github.com/casey/just)

## Features

### Map & markers
- **Interactive plot map** — upload a photo of your garden and draw markers (points, circles, lines, rectangles, freehand paths, and area polygons) over it with zoom and pan
- **Marker types** — assign categories (Tree, Bush, Vegetable, Herb, Path, Structure, etc.) and layers (Water, Electrical, Irrigation, etc.) to each marker
- **Plant details** — date planted, end date, and scientific taxonomy (genus, species, cultivar) for plant-category markers
- **Bulk editing** — Shift+select 2+ markers to batch-update category, layer, and dates
- **Date filter** — scrub the map to any date to see which plants were active at that point in time; the background photo updates to the closest available photo for that date

### Journal & media
- **Journal entries** — date-stamped notes with photo attachments per marker, sorted newest-first
- **Photo lightbox** — click any entry photo to view it full-size
- **Delete entries** — remove a journal note and all its photos in one action

### Harvests
- **Harvest logging** — weight and notes per harvest per individual marker
- **Plant groups** — Shift+select multiple markers to name them as a group and log group-level harvests

### Plot photos & remap
- **Update plot photo** — upload a new overhead photo at any time and associate it with a capture date
- **Homography remap** — pick 4 matching landmarks on the old and new photos; all marker coordinates are automatically transformed to the new perspective using a Direct Linear Transform
- **Live preview** — see where markers will land on the new photo before committing; out-of-bounds markers are highlighted in red
- **One-step undo** — restore the previous photo and marker positions if the remap looks wrong
- **Photo slideshow** — the background image tracks the date filter, showing the most recent plot photo taken on or before the selected date

### Infrastructure
- **Weather log** — rainfall, temperature, and wind records per plot
- **Filters** — filter the map by category or layer
- **Categories & layers** — fully editable with custom colors

## Setup

**Prerequisites**: Go 1.22+, [just](https://github.com/casey/just)

```sh
git clone <repo-url>
cd plotter
just build
just start
```

Then open [http://localhost:8080](http://localhost:8080).

### Development (live reload)

Install [air](https://github.com/air-verse/air):

```sh
go install github.com/air-verse/air@latest
just dev
```

## Configuration

The binary is configured via environment variables. All have sensible defaults for local development.

| Variable | Default | Description |
|----------|---------|-------------|
| `PLOTTER_PORT` | `8080` | Port the HTTP server listens on |
| `PLOTTER_DB` | `plotter.db` | Path to the SQLite database file |
| `PLOTTER_UPLOAD_DIR` | `uploads` | Directory for uploaded plot and marker images |

## Task runner

```
just build      # compile binary
just run        # go run (no build step)
just start      # build + run
just restart    # kill existing, rebuild, run
just dev        # live reload with air
just test       # go test ./...
just test-v     # verbose tests
just tidy       # go mod tidy
just clean      # remove binary
just drop-db    # delete database (irreversible)
just open       # open app in browser
just status     # show what's on port 8080
```

### Deployment recipes

```
just install          # build + copy binary and assets to /opt/plotter
just install-service  # install + register and start the systemd service
just deploy           # rebuild and restart a running service
just backup           # run a manual database and uploads backup
```

## Deploying on a server

The `deploy/` directory contains ready-to-use configuration files for a Linux server deployment.

### systemd service

`deploy/plotter.service` runs Plotter as a system service under a dedicated `plotter` user.

```sh
# Create the user and data directories
sudo useradd --system --no-create-home plotter
sudo mkdir -p /opt/plotter/data/uploads/plots /opt/plotter/data/uploads/markers /opt/plotter/data/backups
sudo chown -R plotter:plotter /opt/plotter/data

# Install everything and enable the service
sudo just install-service
```

To update after a code change:

```sh
sudo just deploy
```

To check logs:

```sh
sudo journalctl -u plotter -f
```

### nginx reverse proxy

`deploy/nginx.conf` is a drop-in nginx config that terminates TLS and proxies to the local server.

```sh
sudo cp deploy/nginx.conf /etc/nginx/sites-available/plotter
# Edit server_name and certificate paths, then:
sudo ln -s /etc/nginx/sites-available/plotter /etc/nginx/sites-enabled/plotter
sudo nginx -t && sudo systemctl reload nginx
```

### Backups

`deploy/backup.sh` backs up the SQLite database (using a live-safe online backup) and the uploads directory. It prunes copies older than 30 days by default.

Add it to cron to run nightly:

```sh
sudo crontab -e
# Add:
0 3 * * * python3 /opt/plotter/deploy/backup.py >> /var/log/plotter-backup.log 2>&1
```

Run a manual backup at any time:

```sh
just backup
# or directly:
sudo python3 /opt/plotter/deploy/backup.py
```

Backup behaviour is controlled by environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PLOTTER_BACKUP_DIR` | `/opt/plotter/data/backups` | Where backups are written |
| `PLOTTER_BACKUP_KEEP_DAYS` | `30` | Days of backups to retain |

## Project structure

```
plotter/
├── db/              # database layer (schema, migrations, all queries)
├── deploy/
│   ├── backup.sh        # cron-friendly backup script
│   ├── nginx.conf        # nginx reverse proxy config
│   └── plotter.service  # systemd unit file
├── handlers/        # HTTP handlers
├── static/
│   ├── css/         # styles
│   ├── js/          # PlotCanvas class (canvas drawing, interaction)
│   └── icons/       # favicon
├── templates/
│   ├── partials/    # HTMX partial templates
│   └── *.html       # full page templates
├── uploads/
│   └── markers/     # uploaded marker photos (local dev only)
├── main.go
└── justfile
```

## Data model

- **Plot** — a garden with a name, address, and current background image
- **PlotImage** — a dated snapshot of a plot photo; the map shows the most recent one on or before the active date filter
- **Marker** — a shape on the plot image (point, circle, line, rect, path, area) with optional category, layer, dates, and group
- **Category** — typed label with color (`plant` or `other`)
- **Layer** — infrastructure layer with color
- **MarkerEntry** — dated journal entry with optional photo attachments
- **PlantTaxonomy** — genus/species/cultivar for a plant marker
- **Harvest** — weight + notes for an individual marker harvest
- **PlantGroup** — named collection of markers; supports group harvest logging
- **Weather** — dated climate records for a plot

## Notes

- No authentication is included — run locally or behind a reverse proxy restricted to trusted networks.
- `CGO_ENABLED=0` is required on some macOS versions to produce a valid binary (set in `justfile` and `.air.toml`).
- The SQLite database is a single file. Back it up regularly — the backup script handles this automatically when run via cron.
