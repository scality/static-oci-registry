FROM golang:1.26.4-alpine@sha256:f1ddd9fe14fffc091dd98cb4bfa999f32c5fc77d2f2305ea9f0e2595c5437c14 AS builder

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
