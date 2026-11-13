FROM mcr.microsoft.com/devcontainers/go:1.27-trixie@sha256:c83c320b68e27dc36903cd5ee07394870c5330dfd77b0897e78c7e1aa00ddfd0

# make testfs needs skopeo.
RUN apt-get update \
  && apt-get install -y --no-install-recommends skopeo \
  && rm -rf /var/lib/apt/lists/*

USER vscode

# Same version as the CI mockery step.
# renovate: datasource=github-releases depName=vektra/mockery
ARG MOCKERY_VERSION=v3.9.0
RUN go install github.com/vektra/mockery/v3@${MOCKERY_VERSION} \
  && go clean -modcache -cache
