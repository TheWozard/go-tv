binary := "go-tv"
image  := "go-tv"
tar    := "go-tv.tar"

# Build the production Go binary (linux/amd64) after generating templates and bundling JS
build: bundle generate
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o {{binary}} main.go

# Install npm dependencies and produce minified JS bundles in static/
bundle:
    npm install
    npm run build

# Remove all gitignored files and directories (node_modules, binaries, bundles, etc.)
clean:
    git clean -fdX

# Run the Go dev server after regenerating templates (run dev-js in a separate terminal for JS watching)
dev: generate
    go run main.go

# Watch JS source files and rebuild bundles on change (run alongside dev)
dev-js:
    npm run watch

# Build and push a Docker image, then export it to a tar file
docker-export: docker-build
    docker save {{image}} -o {{tar}}
    chmod 644 {{tar}}

# Build a linux/amd64 Docker image
docker-build:
    docker build --platform linux/amd64 -t {{image}} .

# Run the Docker image locally on port 8080
docker-run: docker-build
    docker run --rm -p 8080:8080 {{image}}

# Regenerate Go source files from templ templates
generate:
    templ generate ./internal/ui/...

# Generate favicon assets from the source SVG
icons:
    ./scripts/gen-favicon.sh

# Run all Go tests
test:
    go test ./...
