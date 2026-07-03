# ---- builder ----
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache nodejs npm imagemagick imagemagick-svg bash git
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

COPY package.json package-lock.json ./
RUN npm ci

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN git config --global --add safe.directory /app
RUN npm run build && ./scripts/gen-favicon.sh

RUN templ generate ./internal/ui/...

# Strip debug info and DWARF tables to reduce binary size.
RUN CGO_ENABLED=0 go build -ldflags="-w" -o go-tv .

# ---- final image ----
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/go-tv /go-tv

ENTRYPOINT ["/go-tv"]
