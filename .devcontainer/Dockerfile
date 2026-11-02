FROM mcr.microsoft.com/devcontainers/go:1.27-trixie@sha256:32aad7e9eefed4fe99987cc4efef2f9723bd57c556f171143ecf65e0ba855ff3

# make testfs needs skopeo.
RUN apt-get update \
  && apt-get install -y --no-install-recommends skopeo \
  && rm -rf /var/lib/apt/lists/*

USER vscode

# Same version as the CI mockery step.
# renovate: datasource=github-releases depName=vektra/mockery
ARG MOCKERY_VERSION=v3.8.1
RUN go install github.com/vektra/mockery/v3@${MOCKERY_VERSION} \
  && go clean -modcache -cache
