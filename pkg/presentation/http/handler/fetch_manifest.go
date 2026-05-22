package handler

import (
	"log/slog"
	"net/http"

	"github.com/scality/static-oci-registry/pkg/usecase"
)

type FetchManifest struct {
	logger                         *slog.Logger
	fetchManifestFromTagUseCase    *usecase.FetchManifestFromTag
	fetchManifestFromDigestUseCase *usecase.FetchManifestFromDigest
}

func NewFetchManifest(
	logger *slog.Logger,
	fetchManifestFromTagUseCase *usecase.FetchManifestFromTag,
	fetchManifestFromDigestUseCase *usecase.FetchManifestFromDigest,
) *FetchManifest {
	return &FetchManifest{
		logger:                         logger,
		fetchManifestFromTagUseCase:    fetchManifestFromTagUseCase,
		fetchManifestFromDigestUseCase: fetchManifestFromDigestUseCase,
	}
}

func (h *FetchManifest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: implement this handler
}
