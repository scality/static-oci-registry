package domain

import (
	"regexp"
	"strings"
)

// these validation regular expressions are pulled from the OCI distribution spec
// https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#workflow-categories
const (
	// match an image name
	// e.g. myimage, my_image, my-image, my.image, myimage123, my_repo/myimage, my_repo.io/my_image
	imageNameRegex = `^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`


	// match a tag name
	// e.g. latest, v1.0.0, 1.0.0-beta, my_tag-123
	imageTagRegex  = `^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*(\/[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*)*$`
)

var (
	imageNameChecker = regexp.MustCompile(imageNameRegex)
	imageTagChecker = regexp.MustCompile(imageTagRegex)
)

type (
	ImageName string
	Tag       string

	// this is the output format defined in the OCI distribution spec
	// https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-tags
	ListTagsOutput struct {
		Name ImageName `json:"name"`
		Tags []Tag     `json:"tags"`
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

func (i ImageName) String() string {
	return string(i)
}

func (t Tag) String() string {
	return string(t)
}

// from the OCI distribution spec:
// ` If the list is not empty, the tags MUST be in lexical order 
// (i.e. case-insensitive alphanumeric order).`
// https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-tags
func CompareTags(i, j Tag) int {
	if strings.ToLower(string(i)) < strings.ToLower(string(j)) {
		return -1
	}

	if strings.ToLower(string(i)) > strings.ToLower(string(j)) {
		return 1
	}

	return 0
}
