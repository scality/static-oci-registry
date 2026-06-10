FROM golang:1.26.2-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder

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
