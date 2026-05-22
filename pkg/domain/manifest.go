package domain

import "github.com/scality/go-errors"

const schemaVersion = 2

type (
	ManifestDescriptor struct {
		MediaType    string            `json:"mediaType"`
		Digest       Digest            `json:"digest"`
		Size         int64             `json:"size"`
		URLs         []string          `json:"urls,omitempty"`
		Annotations  map[string]string `json:"annotations,omitempty"`
		Data         string            `json:"data,omitempty"`
		ArtifactType string            `json:"artifactType,omitempty"`
	}

	Manifest struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		ArtifactType  string `json:"artifactType,omitempty"`
		// config is non-optional for image manifests but optional for other artifacts
		// this is a pull-only registry, so this shouldn't be an issue to keep non-optional
		Config      *ManifestDescriptor  `json:"config"`
		Layers      []ManifestDescriptor `json:"layers"`
		Subject     *ManifestDescriptor  `json:"subject,omitempty"`
		Annotations map[string]string    `json:"annotations,omitempty"`
	}
)

func (m ManifestDescriptor) Validate() error {
	if m.MediaType == "" {
		return ErrInvalidManifestDescriptor
	}

	if err := m.Digest.Validate(); err != nil {
		return errors.Wrap(
			ErrInvalidManifestDescriptor,
			errors.CausedBy(err),
		)
	}

	return nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != schemaVersion || m.MediaType == "" || m.Config == nil {
		return ErrInvalidManifest
	}

	if err := m.Config.Validate(); err != nil {
		return errors.Wrap(
			ErrInvalidManifest,
			errors.CausedBy(err),
		)
	}

	for _, layer := range m.Layers {
		err := layer.Validate()
		if err != nil {
			return errors.Wrap(
				ErrInvalidManifest,
				errors.CausedBy(err),
			)
		}
	}

	if m.Subject != nil {
		if err := m.Subject.Validate(); err != nil {
			return errors.Wrap(
				ErrInvalidManifest,
				errors.CausedBy(err),
			)
		}
	}

	return nil
}
