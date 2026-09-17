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
> **TLS certificate renewal.** The OCI listener is served over mandatory TLS;
> the metrics listener is served over TLS by default and can be switched to
> plain HTTP via `METRICS_SECURE=false`. Each TLS listener has its own
> independent certificate/key pair, watched on disk
> (`pkg/infrastructure/certwatcher`). The `tls.Config` resolves the certificate
> per-handshake via `GetCertificate`, so renewed certificates are picked up
> **without restarting the server**. Reloads are triggered by `fsnotify`
> filesystem events with a periodic re-read as a safety net, and the watcher
> re-establishes its watch after atomic swaps (e.g. Kubernetes secret
> rotations). It is a lightweight, dependency-free replacement for
> `sigs.k8s.io/controller-runtime/pkg/certwatcher`.

> [!NOTE]
> **HTTP metrics.** The `/v2/` subtree is wrapped by a middleware
> (`pkg/presentation/http/metricsmw`) that composes `promhttp`'s counter and
> duration instrumenters. Endpoint and solution labels are carried through the
> request via a mutable bag (`pkg/presentation/http/reqlabels`) installed on
> the request context: the router sets `Endpoint` on route match, and the
> `FetchManifest` / `PullBlob` handlers write `SolutionName`/`SolutionVersion`
> after the usecase returns. `service.Layout` exposes `SolutionVersion()` so
> infrastructure adapters can populate the winning `(solution, version)` tuple
> on their outputs without string-parsing the layout path. The `registry`
> label is resolved once at startup from `REGISTRY_NAME`
> (`os.Hostname()` fallback) and curried into the vecs.
