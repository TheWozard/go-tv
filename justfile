binary := "go-tv"
image  := "go-tv"
tar    := "go-tv.tar"

# Generate favicon assets from source SVG
icons:
    ./scripts/gen-favicon.sh

# Generate templ components
generate:
    templ generate ./internal/ui/...

# Build the Go binary
build: generate
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o {{binary}} main.go

# Development mode - run Go server
dev: generate
    go run main.go

# Build Docker image
docker-build:
    docker build --platform linux/amd64 -t {{image}} .

# Export Docker image to tar file
docker-export: docker-build
    docker save {{image}} -o {{tar}}
    chmod 644 {{tar}}

# Run Docker image
docker-run: docker-build
    docker run --rm -p 8080:8080 {{image}}

# Clean build artifacts
clean:
    rm -f {{binary}}
    rm -f {{tar}}

test:
    go test ./...
