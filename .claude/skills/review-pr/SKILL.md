---
name: review-pr
description: Review a PR on static-oci-registry (Go read-only OCI container registry serving the OCI distribution spec from a filesystem over TLS)
argument-hint: <pr-number-or-url>
disable-model-invocation: true
allowed-tools: Read, Bash(gh repo view *), Bash(gh pr view *), Bash(gh pr diff *), Bash(gh pr comment *), Bash(gh api *), Bash(git diff *), Bash(git log *), Bash(git show *)
---

# Review GitHub PR

You are an expert code reviewer. Review this PR: $ARGUMENTS

## Determine PR target

Parse `$ARGUMENTS` to extract the repo and PR number:

- If arguments contain `REPO:` and `PR_NUMBER:` (CI mode), use those values directly.
- If the argument is a GitHub URL (starts with `https://github.com/`), extract `owner/repo` and the PR number from it.
- If the argument is just a number, use the current repo from `gh repo view --json nameWithOwner -q .nameWithOwner`.

## Output mode

- **CI mode** (arguments contain `REPO:` and `PR_NUMBER:`): post inline comments and summary to GitHub.
- **Local mode** (all other cases): output the review as text directly. Do NOT post anything to GitHub.

## Steps

1. **Fetch PR details:**

```bash
gh pr view <number> --repo <owner/repo> --json title,body,headRefOid,author,files
gh pr diff <number> --repo <owner/repo>
```

2. **Read changed files** to understand the full context around each change (not just the diff hunks).

3. **Analyze the changes** against these criteria:

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

4. **Deliver your review:**

### If CI mode: post to GitHub

#### Part A: Inline file comments

For each issue, post a comment on the exact file and line. Keep comments short (1-3 sentences), end with `— Claude Code`. Use line numbers from the **new version** of the file.

**Without suggestion block** — single-line command, `<br>` for line breaks:
```bash
gh api -X POST -H "Accept: application/vnd.github+json" "repos/<owner/repo>/pulls/<number>/comments" -f body="Issue description.<br><br>— Claude Code" -f path="file" -F line=42 -f side="RIGHT" -f commit_id="<headRefOid>"
```

**With suggestion block** — use a heredoc (`-F body=@-`) so code renders correctly:
```bash
gh api -X POST -H "Accept: application/vnd.github+json" "repos/<owner/repo>/pulls/<number>/comments" -F body=@- -f path="file" -F line=42 -f side="RIGHT" -f commit_id="<headRefOid>" <<'COMMENT_BODY'
Issue description.

```suggestion
first line of suggested code
second line of suggested code
```

— Claude Code
COMMENT_BODY
```

Only suggest when you can show the exact replacement. For architectural or design issues, just describe the problem.

#### Part B: Summary comment

Single-line command, `<br>` for line breaks. No markdown headings — they render as giant bold text. Flat bullet list only:

```bash
gh pr comment <number> --repo <owner/repo> --body "- file:line — issue<br>- file:line — issue<br><br>Review by Claude Code"
```

If no issues: just say "LGTM". End with: `Review by Claude Code`

### If local mode: output the review as text

Do NOT post anything to GitHub. Instead, output the review directly as text.

For each issue found, output:

```
**<file_path>:<line_number>** — <what's wrong and how to fix it>
```

When the fix is a concrete line change, include a fenced code block showing the suggested replacement.

At the end, output a summary section listing all issues. If no issues: just say "LGTM".

End with: `Review by Claude Code`

## What NOT to do

- Do not comment on markdown formatting preferences
- Do not suggest refactors unrelated to the PR's purpose
- Do not praise code — only flag problems or stay silent
- If no issues are found, post only a summary saying "LGTM"
- Do not flag style issues already covered by the project's linter (golangci-lint)
