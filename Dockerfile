FROM gcr.io/distroless/static-debian12

COPY go-tv /go-tv

ENTRYPOINT ["/go-tv"]
