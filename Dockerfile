# ---- builder ----
FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache nodejs npm imagemagick imagemagick-svg bash
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001

COPY package.json package-lock.json ./
RUN npm ci

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN npm run build && ./scripts/gen-favicon.sh

RUN templ generate ./internal/ui/...

# Strip debug info and DWARF tables to reduce binary size.
RUN CGO_ENABLED=0 go build -ldflags="-w" -o go-tv .

# ---- final image ----
FROM alpine:latest

RUN apk --no-cache add ca-certificates iptables

WORKDIR /root/

COPY --from=builder /app/go-tv .

EXPOSE 443

ENTRYPOINT ["./go-tv"]
