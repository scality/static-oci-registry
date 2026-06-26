package domain

// Media type identifiers for OCI image manifests and indexes, and their Docker
// equivalents. The authoritative list of OCI media types (and the Docker
// compatibility matrix) lives in the OCI image-spec:
// https://github.com/opencontainers/image-spec/blob/main/media-types.md
const (
	MediaTypeOCIImageManifest   = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeOCIImageIndex      = "application/vnd.oci.image.index.v1+json"
	MediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"

	// RefNameAnnotation is the OCI annotation that carries a tag name in an
	// image index entry (image-spec image-layout.md).
	RefNameAnnotation = "org.opencontainers.image.ref.name"
)

// IsImageIndexMediaType reports whether mt is an image index / manifest list
// type (OCI or Docker).
func IsImageIndexMediaType(mt string) bool {
	return mt == MediaTypeOCIImageIndex || mt == MediaTypeDockerManifestList
}
