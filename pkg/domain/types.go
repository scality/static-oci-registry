package domain

import (
	"regexp"
	"strings"
)

// these validation regular expressions are pulled from the OCI distribution spec
// https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#workflow-categories
const (
	// match an image name
	// e.g. myimage, my_image, my-image, my.image, myimage123, my_repo/myimage, my_repo.io/my_image.
	imageNameRegex = `^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`

	// match a tag name
	// e.g. latest, v1.0.0, 1.0.0-beta, my_tag-123.
	imageTagRegex = `^[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}$`

	// match a digest
	// digests follow this specific grammar
	// digest                ::= algorithm ":" encoded
	// algorithm             ::= algorithm-component (algorithm-separator algorithm-component)*
	// algorithm-component   ::= [a-z0-9]+
	// algorithm-separator   ::= [+._-]
	// encoded               ::= [a-zA-Z0-9=_-]+
	// e.g. sha256:02c91f6b395a6b06d17b8985a2db90aef7b09feedff77eff1f7c269161263a9b
	// e.g. multihash+base58:QmRZxt2b1FVZPNqd8hsiykDL3TdBDeTSPX9Kv46HmX4Gx8
	// we do not need to check the validity of the hash value, only the grammar
	// cf. https://github.com/opencontainers/image-spec/blob/main/descriptor.md#digests
	digestEncodedRegex            = `[a-zA-Z0-9=_-]+`
	digestAlgorithmSeparatorRegex = `[+._-]`
	digestAlgorithmComponentRegex = `[a-z0-9]+`
	digestAlgorithmRegex          = digestAlgorithmComponentRegex +
		`(` + digestAlgorithmSeparatorRegex + digestAlgorithmComponentRegex + `)*`
	digestRegex = `^(?P<algorithm>` + digestAlgorithmRegex +
		`):(?P<encoded>` + digestEncodedRegex + `)$`
)

var (
	imageNameChecker = regexp.MustCompile(imageNameRegex)
	imageTagChecker  = regexp.MustCompile(imageTagRegex)
	digestChecker    = regexp.MustCompile(digestRegex)
)

type (
	SolutionVersion struct {
		Solution string
		Version  string
	}

	TagEntry struct {
		SolutionVersion
		Name ImageName
		Tag  Tag
	}

	ImageName string
	Tag       string
	Digest    string

	ListTagsInput struct {
		Name ImageName
		N    *int
		Last *Tag
	}

	// this is the output format defined in the OCI distribution spec
	// https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#listing-tags
	ListTagsOutput struct {
		Name ImageName `json:"name"`
		Tags []Tag     `json:"tags"`
	}

	ManifestReference interface {
		isManifestReference()
		String() string
		Validate() error
	}

	FetchManifestInput struct {
		Name ImageName
		Ref  ManifestReference
		Head bool
	}

	FetchManifestOutput struct {
		MediaType     string
		ContentDigest Digest
		ManifestBytes []byte
	}
)

func (i ImageName) Validate() error {
	if !imageNameChecker.Match([]byte(i)) {
		return ErrInvalidImageName
	}

	return nil
}

func (t Tag) Validate() error {
	if !imageTagChecker.Match([]byte(t)) {
		return ErrInvalidTag
	}

	return nil
}

func (d Digest) Validate() error {
	if !digestChecker.Match([]byte(d)) {
		return ErrInvalidDigest
	}

	return nil
}

func (d Digest) Algorithm() (string, error) {
	matches := digestChecker.FindStringSubmatch(string(d))
	if matches == nil {
		return "", ErrInvalidDigest
	}

	return matches[digestChecker.SubexpIndex("algorithm")], nil
}

func (d Digest) Encoded() (string, error) {
	matches := digestChecker.FindStringSubmatch(string(d))
	if matches == nil {
		return "", ErrInvalidDigest
	}

	return matches[digestChecker.SubexpIndex("encoded")], nil
}

func (i ImageName) String() string {
	return string(i)
}

func (t Tag) String() string {
	return string(t)
}

func (Tag) isManifestReference() {}

func (d Digest) String() string {
	return string(d)
}

func (Digest) isManifestReference() {}

// from the OCI distribution spec:
// ` If the list is not empty, the tags MUST be in lexical order
// (i.e. case-insensitive alphanumeric order).`
// https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#listing-tags
func CompareTags(i, j Tag) int {
	if strings.ToLower(string(i)) < strings.ToLower(string(j)) {
		return -1
	}

	if strings.ToLower(string(i)) > strings.ToLower(string(j)) {
		return 1
	}

	return 0
}
