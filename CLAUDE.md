# static-oci-registry

This is a **Go implementation of a read-only OCI container registry** serving the
[OCI distribution spec](https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md)
Pull and Discovery use cases. Images are read from a filesystem hierarchy
(`<solutions>/<solution>/<version>/`) and served over mandatory TLS.

It contains:

- **Clean architecture** layers under `pkg/`:
  - `pkg/domain/` — entities, types, and sentinel errors (`pkg/domain/errors.go`, `pkg/domain/ocierrors/`)
  - `pkg/usecase/` — application use cases (list tags, fetch manifest by tag/digest)
  - `pkg/service/` — service interfaces (ports)
  - `pkg/infrastructure/` — adapters: filesystem implementations, DI container (`pkg/infrastructure/di/`)
  - `pkg/presentation/http/` — HTTP router, handlers, and response helpers
- Entry point and config in `cmd/` (`cmd/main.go`, `cmd/config/environment.go`, env via `go-envconfig`)
- Tests in `test/` (`unit/`, `integration/`, `utils/`) using **Ginkgo/Gomega**

Conventions:

- Errors are wrapped with `github.com/scality/go-errors` (`errors.Wrap`, `errors.WithDetail`),
  not the stdlib `fmt.Errorf`. Domain sentinel errors live in `pkg/domain/errors.go`.
- Logging uses stdlib `log/slog` with context (`InfoContext`, etc.) and structured attributes.
- `go-errors` is the only Scality dependency; it is a tagged Go module in `go.mod`, not a vendored git branch.
- Linting is enforced by `golangci-lint` (see `.golangci.yaml`).
