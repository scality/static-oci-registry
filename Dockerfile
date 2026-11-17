FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder

ARG APPLICATION_VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags "-X 'github.com/scality/static-oci-registry/cmd/config.ApplicationVersion=${APPLICATION_VERSION}'" -o static-oci-registry ./cmd/main.go

FROM scratch

LABEL org.opencontainers.image.source=https://github.com/scality/static-oci-registry

COPY --from=builder /app/static-oci-registry /bin/static-oci-registry

ENTRYPOINT ["/bin/static-oci-registry"]
