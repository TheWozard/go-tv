# ---- builder ----
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache nodejs npm
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

COPY package.json package-lock.json ./
RUN npm ci && cp node_modules/@starfederation/datastar/dist/datastar.js /tmp/datastar.js

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN mkdir -p static/vendor && cp /tmp/datastar.js static/vendor/datastar.js

RUN templ generate ./internal/ui/...

# Strip debug info and DWARF tables to reduce binary size.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o go-tv .

# ---- final image ----
FROM scratch

WORKDIR /app

COPY --from=builder /app/go-tv .

# Mount a volume at /app to persist state.json across restarts,
# or bind-mount just /app/state.json.
VOLUME ["/app"]

EXPOSE 8080
ENTRYPOINT ["./go-tv"]
