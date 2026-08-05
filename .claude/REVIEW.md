# Review criteria

Read by the `/review-pr` skill (Scality agent hub) and by anyone reviewing by hand.
Flag problems only — see "What not to flag" at the end.

## What this repo is

`static-oci-registry` is a read-only OCI container registry: it serves the OCI
distribution spec v1.0.1 straight from a filesystem tree, over TLS. Layers follow
clean architecture — `pkg/domain` depends on nothing, `pkg/usecase` on `service`
ports, HTTP and filesystem live in `presentation`/`infrastructure`, wiring in
`pkg/infrastructure/di/`.

## Criteria

| Area | What to check |
|------|---------------|
| Error wrapping | Errors must be wrapped with `github.com/scality/go-errors` (`errors.Wrap`, `errors.WithDetail`), not stdlib `fmt.Errorf`/`%w`. New failure modes that map to a domain concept should use or add a sentinel in `pkg/domain/errors.go`. No swallowed errors. |
| OCI spec compliance | Responses must follow the OCI distribution spec v1.0.1: correct status codes, `Docker-Content-Digest` and `Content-Type` headers, empty body for `HEAD`, correct OCI error codes/JSON (`pkg/domain/ocierrors/`). Pagination params `n`/`last` on `tags/list` must behave per spec. |
| Clean architecture | Respect layer boundaries: `domain` depends on nothing; `usecase` depends on `service` ports, not infrastructure; `presentation` and `infrastructure` are adapters. No leaking of HTTP/filesystem types into domain or use cases. New ports wired through `pkg/infrastructure/di/`. |
| Context propagation | `context.Context` must be threaded through call chains and passed to `slog` (`InfoContext`, etc.) and downstream calls; respect cancellation. |
| Logging | Use stdlib `log/slog` with structured attributes and context; no `fmt.Println`/`log.Printf` in production code; log levels match severity; do not log secrets or full filesystem paths unnecessarily. |
| Filesystem & path safety | Reads must stay within `FS_ROOT`; guard against path traversal from user-supplied image names/references; handle missing/corrupt entries gracefully (soft-fail per-entry, not whole-request crash). |
| TLS & config | TLS stays mandatory (service must refuse to start without cert/key, min TLS 1.2). Env var changes in `cmd/config/environment.go` must keep backward compatibility, sane defaults, and consistent naming. |
| Interface compliance | Implementations should satisfy their service interface, ideally asserted at compile time (`var _ service.X = (*impl)(nil)`). |
| Goroutine & resource safety | Goroutines have clear exit conditions; opened files/readers are closed on all paths (including error paths); no leaked descriptors. |
| Dependency pinning | `go.mod`/`go.sum` changes must pin Scality deps (`go-errors`) to a tagged release, never a branch or pseudo-version pointing at a moving ref. |
| Security | Input validation on image names, tags, digests (`sha256:<hex>`); no injection via path or headers; no credentials/keys committed. |
| Breaking changes | Anything changing the HTTP API contract, public Go interfaces, or env var behaviour. |
| Tests | New behaviour covered by Ginkgo/Gomega tests under `test/`; spec-relevant edge cases (not-found, invalid reference, pagination bounds) exercised. |

## What not to flag

- Anything the linters already own: `golangci-lint`, `gofmt`, `goimports` —
  formatting, import order, unused variables, naming.
- Markdown or comment wording preferences.
- Refactors unrelated to the PR's purpose.
