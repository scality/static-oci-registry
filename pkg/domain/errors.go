package domain

import (
	"github.com/scality/go-errors"
)

var (
	ErrInvalidImageName          = errors.New("invalid image name")
	ErrImageNotFound             = errors.New("image not found in registry")
	ErrInvalidTag                = errors.New("invalid tag")
	ErrTagNotFound               = errors.New("tag not found in image")
	ErrInvalidDigest             = errors.New("invalid digest")
	ErrInvalidManifestDescriptor = errors.New("invalid manifest descriptor")
	ErrInvalidManifest           = errors.New("invalid manifest")
	ErrRegistryInternal          = errors.New("there was an internal error in the registry")
	ErrInvalidParameter          = errors.New("invalid parameter")
	ErrParameterOutOfRange       = errors.New("parameter out of range")
)
