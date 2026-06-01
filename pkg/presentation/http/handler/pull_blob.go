package handler

import (
	"log/slog"
	"net/http"
	"regexp"

	"github.com/scality/static-oci-registry/pkg/usecase"
)

const pullBlobURLPattern = `^/v2/(.+)/blobs/([^/]+)$`

var pullBlobURLRegex = regexp.MustCompile(pullBlobURLPattern)

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

func (*PullBlob) Matches(path string) bool {
	return pullBlobURLRegex.MatchString(path)
}

func (h *PullBlob) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}
