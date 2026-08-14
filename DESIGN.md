# Design Static Registry Server

## Goal
The goal of the Static Registry is to provide an OCI registry that follows the Pull and Discovery
use cases of the [OCI distribution spec](https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#endpoints).

This registry should serve using TLS the images stored in a specified filesystem path (`<solutions>`) that
follows this hierarchy:
```
<solutions>/
    solution1/
        1.0.0/
        1.2.0/
        2.0.0/
    solution2/
        3.5.9/
        4.2.0/
```

> [!NOTE]
> In this implementations, the versions numbers are interpreted using SemVer for sorting.
> In order to have a properly functioning sort algorithm, it is strongly advised to use
> [valid semver](https://semver.org) version numbers in all solutions

When an image `image1` from solution `mysolution` is requested, using reference `myregistry.lan/mysolution/image1:v1.2`
then the registry should iterate over every version sub-directory of `<solutions>/mysolution/` until it finds
one that contains the requested image.

The same logic must be followed when implementing registry discovery endpoints (getting manifests and referrers)

## Implementation details
This registry is a server built using Go. It uses a clean architecture paradigm.
for now, it only supports reading registry contents from a filesystem but the clean architecture makes
it extensible for other kinds of content sources.

> [!NOTE]
> **On-disk format.** Each leaf `<image>/` directory is an
> [OCI Image Layout](https://github.com/opencontainers/image-spec/blob/main/image-layout.md) -
> an `oci-layout` marker file, an `index.json`, and a shared `blobs/` store - not per-tag
> directories. Tags are expressed as `org.opencontainers.image.ref.name` annotations on
> entries in `index.json`.
>
> Multi-arch images are served by returning the image index at the tag endpoint; clients
> resolve the per-platform manifest by digest in a follow-up request. The registry never
> selects a platform - that is the client's responsibility.
>
> All reads are performed on demand from disk (disk is the source of truth), so solutions,
> versions, images, and tags can be added or removed without restarting the server.

> [!NOTE]
> **TLS certificate renewal.** The server is served over mandatory TLS, and the
> certificate and key files are watched on disk (`pkg/infrastructure/certwatcher`).
> The `tls.Config` resolves the certificate per-handshake via `GetCertificate`, so
> renewed certificates are picked up **without restarting the server**. Reloads are
> triggered by `fsnotify` filesystem events with a periodic re-read as a safety net,
> and the watcher re-establishes its watch after atomic swaps (e.g. Kubernetes secret
> rotations). It is a lightweight, dependency-free replacement for
> `sigs.k8s.io/controller-runtime/pkg/certwatcher`.
