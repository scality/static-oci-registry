[![Post Merge](https://github.com/scality/static-oci-registry/actions/workflows/post-merge.yaml/badge.svg)](https://github.com/scality/static-oci-registry/actions/workflows/post-merge.yaml)
[![GitHub release](https://img.shields.io/github/v/release/scality/static-oci-registry)](https://github.com/scality/static-oci-registry/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/scality/static-oci-registry)](go.mod)
[![License](https://img.shields.io/github/license/scality/static-oci-registry)](LICENSE)

# Static Container Registy

A container registry implementation that respects [OCI spec](https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md)
for Pull and Discovery. Written in Go.

> [!NOTE]
> This registry targets the **OCI distribution-spec v1.0.1** rather than
> [v1.1](https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md).
> Restricted to the Pull and Content Discovery workflows that this service
> implements, v1.1 only adds two things on top of v1.0.1:
>
> 1. **`Link` header pagination on `GET /v2/<name>/tags/list`** (RFC 5988
>    `rel="next"`). The `n` and `last` query parameters that drive
>    pagination already exist in v1.0.1, and clients fall back to them when
>    no `Link` header is returned, so omitting it is backward compatible.
> 2. **The Referrers API** (`GET /v2/<name>/referrers/<digest>` and the
>    referrers tag-schema fallback). This is used to discover artifacts
>    (signatures, SBOMs, attestations, …) attached to an image via the
>    `subject` field. The use cases served by this registry do not yet need to
>    list referrers for the images it serves, so the endpoint is
>    intentionally not implemented. Spec-compliant clients are expected to
>    treat a missing referrers endpoint as "no referrers" and continue
>    pulling normally.
>
> Push, chunked upload, cross-repo mount, deletion and the `OCI-Subject`
> response header are also v1.1 additions, but they are push/management
> concerns and are out of scope for this read-only registry.

## Endpoints

| Method         | API Endpoint                       | Success     | Failure           | Params      |
| -------------- | ---------------------------------- | ----------- | ----------------- | ----------- |
| `GET`          | `/v2/`                             | `200`       | `404`/`401`       |             |
| `GET`          | `/v2/<name>/tags/list`             | `200`       | `404`             | n, last, ns |
| `GET`, `HEAD`  | `/v2/<name>/manifests/<reference>` | `200`       | `404`             | ns          |
| `GET`, `HEAD`  | `/v2/<name>/blobs/<digest>`        | `200`/`206` | `404`/`416`       | ns          |

`<reference>` may be either a tag or a digest (e.g. `sha256:<hex>`). On success,
responses include the `Docker-Content-Digest` and `Content-Type` headers; `HEAD`
returns the same headers with an empty body.

The `ns` query parameter is accepted on all repository-scoped endpoints for
compatibility with clients such as **containerd**, which split a pull reference
like `docker.io/library/alpine` into a namespace (`ns=docker.io`) and a repository
path (`library/alpine`). When present, `ns` is prepended to `<name>`
before the image is resolved against the on-disk hierarchy; requests
without `ns` are unaffected.

Blob responses stream the content from disk and support `Range` requests
(`206 Partial Content`, `416 Requested Range Not Satisfiable`). A blob is only
served if its `<digest>` is referenced (as `config.digest`, a `layers[].digest`,
or `subject.digest`) by a manifest reachable from the image's `index.json` -
directly, or via an image index for multi-arch images. Stray files in the layout
are never exposed.

When pulling a blob, the `Docker-Content-Digest` response header echoes the digest from the request URL;
the registry trusts the on-disk layout and does not re-hash blobs on the fly, which
also avoids the cost of streaming every byte through a hash function on each pull.

## On-disk format

Each image is stored as an [OCI Image Layout](https://github.com/opencontainers/image-spec/blob/main/image-layout.md)
under `<solutions>/<solution>/<version>/<image>/`: an `oci-layout` marker, an `index.json`
whose entries carry tags via the `org.opencontainers.image.ref.name` annotation, and a
shared `blobs/<algorithm>/<digest>` store.

Multi-architecture images are supported. A multi-arch tag resolves to an image index, which
is what `GET /v2/<name>/manifests/<tag>` returns; the client then selects a platform and
fetches the per-platform manifest by digest. The registry never inspects platforms itself -
it serves the index, any manifest reachable from it by digest, and the blobs those manifests
reference.

Images can be produced with
`skopeo copy --all docker://<ref> oci:<solutions>/<solution>/<version>/<image>:<tag>`
(repeated tags accumulate into one layout with shared blobs). The on-disk hierarchy may be
modified while the server runs - solutions, versions, images and tags are read on demand on
each request, so additions and removals take effect without a restart.

## Environment variables

This service can be configured through environment variables:

| Variable                | Behaviour                                                         |
| ----------------------- | ----------------------------------------------------------------- |
| LOG_LEVEL               | Sets the log level for the service                                |
| HTTP_ADDR               | Sets the address the service listens and serves HTTP requests on  |
| FS_ROOT                 | Sets the path to the root filesystem that the registry reads from |
| HTTP_TLS_CERT_FILE_PATH | Path to the TLS certificate file (PEM). **Required.**            |
| HTTP_TLS_KEY_FILE_PATH  | Path to the TLS private key file (PEM). **Required.**            |

> [!NOTE]
> TLS is mandatory. The service will not start if `HTTP_TLS_CERT_FILE_PATH` or
> `HTTP_TLS_KEY_FILE_PATH` are not set. TLS 1.2 is the minimum accepted version.

### TLS certificate renewal

The certificate and key files are watched on disk and reloaded automatically, so
renewed certificates take effect **without restarting the server**. The
certificate is resolved per-handshake (via `tls.Config.GetCertificate`), meaning
new connections immediately use the latest certificate once it is reloaded.

Reloads are driven by filesystem events (`fsnotify`) and, as a safety net, a
periodic re-read of the files (every 120s). The watcher tolerates atomic swaps
such as Kubernetes secret rotations - when the underlying file is renamed or
replaced, the watch is re-established on the original path. The cached
certificate is only swapped when the certificate or key actually changes.
