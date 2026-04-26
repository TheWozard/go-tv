# ---- builder ----
FROM golang:alpine AS builder

WORKDIR /app

# Install the tdewolff minify CLI for the HTML minification step.
RUN go install github.com/tdewolff/minify/v2/cmd/minify@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Strip debug info and DWARF tables to reduce binary size.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o go-tv .

# Minify HTML in-place.
RUN minify -o static/index.html static/index.html

# ---- final image ----
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/go-tv .
COPY --from=builder /app/static ./static
COPY --from=builder /app/schedule.json .

# Mount a volume at /app to persist state.json across restarts,
# or bind-mount just /app/state.json.
VOLUME ["/app"]

EXPOSE 8080
ENTRYPOINT ["./go-tv"]
