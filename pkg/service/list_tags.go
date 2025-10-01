package service

import "github.com/scality/static-oci-registry/pkg/domain"

type TagLister interface {
	ListTags(imageName domain.ImageName) (*domain.ListTagsOutput, error)
}
