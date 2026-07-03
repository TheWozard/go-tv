set dotenv-load

binary   := "go-tv"
image    := "go-tv"
tar      := "go-tv.tar"
registry := env("REGISTRY", "localhost:8010")

# Build the production Go binary (linux/amd64) after generating templates, icons, and bundling JS
build: bundle icons generate
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o {{binary}} .

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

# Build and push the Docker image to the registry defined in .env
docker-push: docker-build
    docker tag {{image}} {{registry}}/{{image}}
    docker push {{registry}}/{{image}}

# Build and push a Docker image, then export it to a tar file
docker-export: docker-build
    docker save {{image}} -o {{tar}}
    chmod 644 {{tar}}

# Build a linux/amd64 Docker image from the locally built binary
docker-build: build
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
