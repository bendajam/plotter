# Plotter

A personal garden history tracker. Drop markers on an image of your plot, log notes and harvests, track plant taxonomy, and build up a living record of your garden over time.

## Stack

- **Backend**: Go 1.22, [chi](https://github.com/go-chi/chi) router
- **Database**: SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go, no CGO required)
- **Frontend**: [HTMX 2](https://htmx.org/) for server-driven partials, vanilla JS canvas for the map
- **Templates**: Go `html/template`
- **Task runner**: [just](https://github.com/casey/just)

## Features

- **Interactive plot map** — upload a photo of your garden and draw markers (circles, rectangles, polygons) over it with zoom and pan
- **Marker types** — assign categories (Tree, Bush, Vegetable, Herb, Path, Structure, etc.) and layers (Water, Electrical, Irrigation, etc.) to markers
- **Plant details** — date planted, end date, and scientific taxonomy (genus, species, cultivar) for plant-category markers
- **Journal entries** — date-stamped notes with photo attachments per marker
- **Harvest logging** — weight and notes per harvest per marker
- **Plant groups** — select multiple markers with Shift+click, name them as a group, and log group harvests
- **Bulk editing** — Shift+select 2+ markers to batch-update category, layer, and dates
- **Weather log** — rainfall, temperature, wind records per plot
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

## Project structure

```
plotter/
├── db/              # database layer (schema, migrations, all queries)
├── handlers/        # HTTP handlers
├── static/
│   ├── css/         # styles
│   ├── js/          # PlotCanvas class (canvas drawing, interaction)
│   └── icons/       # favicon
├── templates/
│   ├── partials/    # HTMX partial templates
│   └── *.html       # full page templates
├── uploads/
│   └── markers/     # uploaded marker photos
├── main.go
└── justfile
```

## Data model

- **Plot** — a garden with an image, name, and address
- **Marker** — a shape on the plot image (circle, rect, polygon) with optional category, layer, dates, and group
- **Category** — typed label with color (`plant` or `other`)
- **Layer** — infrastructure layer with color
- **MarkerEntry** — dated journal entry with optional images
- **PlantTaxonomy** — genus/species/cultivar for a plant marker
- **Harvest** — weight + notes for an individual marker
- **PlantGroup** — named collection of markers; supports group harvest logging
- **Weather** — dated climate records for a plot

## Notes

- The database file is `plotter.db` in the working directory. Back it up to preserve your data.
- Uploaded images are stored in `uploads/markers/`.
- The server binds to `:8080`. No auth is included — run it locally or behind a reverse proxy.
- `CGO_ENABLED=0` is required on some macOS versions to produce a valid binary (set in `justfile`).
