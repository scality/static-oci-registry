package unit

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"

	"github.com/scality/static-oci-registry/pkg/domain"
)

// validManifestJSON is a minimally-valid OCI image manifest per domain.Manifest.Validate().
// SchemaVersion=2, non-empty MediaType, Config with valid MediaType + digest.
const validManifestJSON = `{` +
	`"schemaVersion":2,` +
	`"mediaType":"application/vnd.oci.image.manifest.v1+json",` +
	`"config":{` +
	`"mediaType":"application/vnd.oci.image.config.v1+json",` +
	`"digest":"sha256:` +
	`0000000000000000000000000000000000000000000000000000000000000000",` +
	`"size":7023` +
	`},` +
	`"layers":[]` +
	`}`

func sha256Digest(bytes []byte) domain.Digest {
	h := sha256.Sum256(bytes)
	return domain.Digest(fmt.Sprintf("sha256:%x", h[:]))
}

func sha512Digest(bytes []byte) domain.Digest {
	h := sha512.Sum512(bytes)
	return domain.Digest(fmt.Sprintf("sha512:%x", h[:]))
}
