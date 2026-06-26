package domain

import "github.com/scality/go-errors"

// Index follows the OCI Image Index (a.k.a. manifest list).
// cf. https://github.com/opencontainers/image-spec/blob/main/image-index.md
// The on-disk index.json of an OCI Image Layout is an Index.
type Index struct {
	SchemaVersion int                  `json:"schemaVersion"`
	MediaType     string               `json:"mediaType"`
	Manifests     []ManifestDescriptor `json:"manifests"`
	Annotations   map[string]string    `json:"annotations,omitempty"`
}

func (i Index) Validate() error {
	if i.SchemaVersion != schemaVersion || !IsImageIndexMediaType(i.MediaType) {
		return ErrInvalidIndex
	}

	for _, m := range i.Manifests {
		if err := m.Validate(); err != nil {
			return errors.Wrap(ErrInvalidIndex, errors.CausedBy(err))
		}
	}

	return nil
}
