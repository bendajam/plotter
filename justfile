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
