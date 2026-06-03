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

| Method         | API Endpoint                       | Success     | Failure           | Params  |
| -------------- | ---------------------------------- | ----------- | ----------------- | ------- |
| `GET`          | `/v2/`                             | `200`       | `404`/`401`       |         |
| `GET`          | `/v2/<name>/tags/list`             | `200`       | `404`             | n, last |
| `GET`, `HEAD`  | `/v2/<name>/manifests/<reference>` | `200`       | `404`             |         |
| `GET`, `HEAD`  | `/v2/<name>/blobs/<digest>`        | `200`/`206` | `404`/`416`       |         |

`<reference>` may be either a tag or a digest (e.g. `sha256:<hex>`). On success,
responses include the `Docker-Content-Digest` and `Content-Type` headers; `HEAD`
returns the same headers with an empty body.

Blob responses stream the content from disk and support `Range` requests
(`206 Partial Content`, `416 Requested Range Not Satisfiable`). A blob is only
served if its `<digest>` is referenced by the manifest of at least one tag
under `<name>` (as `config.digest`, a `layers[].digest`, or `subject.digest`);
stray files in the tag directory are never exposed.

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
