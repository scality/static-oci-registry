package handler

import (
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	httplayer "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/usecase"
)

const pullBlobURLPattern = `^/v2/(.+)/blobs/([^/]+)$`

var pullBlobURLRegex = regexp.MustCompile(pullBlobURLPattern)

type PullBlobInput struct {
	ImageName domain.ImageName
	Digest    domain.Digest
}

type PullBlob struct {
	logger          *slog.Logger
	pullBlobUseCase *usecase.PullBlob
}

func NewPullBlob(
	logger *slog.Logger,
	pullBlobUseCase *usecase.PullBlob,
) *PullBlob {
	return &PullBlob{
		pullBlobUseCase: pullBlobUseCase,
		logger:          logger.With(slog.String("http_handler", "pull_blob")),
	}
}

// Matches reports whether the given request path is served by this handler.
func (*PullBlob) Matches(path string) bool {
	return pullBlobURLRegex.MatchString(path)
}

// AllowedMethods reports the HTTP methods this endpoint accepts.
// end-2 of the OCI distribution-spec accepts GET and HEAD.
func (*PullBlob) AllowedMethods() []string {
	return []string{http.MethodGet, http.MethodHead}
}

func (h *PullBlob) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pullBlobInput, err := parsePullBlobRequest(r)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	rc, err := h.pullBlobUseCase.Execute(ctx, pullBlobInput.ImageName, pullBlobInput.Digest)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	defer rc.Close()

	// must set Content-Type to a generic octet-stream as blobs are opaque to
	// the registry, and must contain the digest of the body in the
	// Docker-Content-Digest header per the OCI distribution-spec.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", pullBlobInput.Digest.String())

	// Delegate body writing to http.ServeContent so that Range requests, HEAD
	// requests, and conditional requests (If-Modified-Since / If-None-Match)
	// are handled correctly without buffering the blob in memory. The empty
	// name and zero modtime disable filename-based content sniffing and the
	// Last-Modified header respectively, both of which are meaningless for a
	// content-addressed blob.
	http.ServeContent(w, r, "", time.Time{}, rc)
}

// parses a query and returns the input type for the PullBlob usecase.
func parsePullBlobRequest(r *http.Request) (*PullBlobInput, error) {
	// Extract image name and digest from URL path
	// Path format: /v2/{image}/blobs/{digest}
	// use a regular expression to extract the image name and digest
	// since the image name can contain multiple levels of slashes
	path := r.URL.Path

	matches := pullBlobURLRegex.FindStringSubmatch(path)
	if matches == nil {
		// Unreachable: the router only dispatches to this handler when
		// PullBlob.Matches(path) returns true, which uses the same regex.
		// A nil result here means the router and handler are out of sync —
		// a programming error, not a client error.
		panic("pull_blob: router/handler regex mismatch for path " + path)
	}

	image := domain.ImageName(matches[1])
	if err := image.Validate(); err != nil {
		return nil, errors.Wrap(
			err,
			errors.WithDetail("error validating image name in query parser"),
			ocierrors.BuildOCIProperties(
				ocierrors.NameInvalid,
				err.Error(),
				map[string]string{"name": image.String()},
			),
		)
	}

	digest := domain.Digest(matches[2])
	if err := digest.Validate(); err != nil {
		return nil, errors.Wrap(
			err,
			errors.WithDetail("error validating digest in query parser"),
			ocierrors.BuildOCIProperties(
				ocierrors.DigestInvalid,
				err.Error(),
				map[string]string{"digest": string(digest)},
			),
		)
	}

	return &PullBlobInput{
		ImageName: image,
		Digest:    digest,
	}, nil
}
