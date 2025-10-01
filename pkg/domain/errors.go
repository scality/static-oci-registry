package domain

import (
	"github.com/pkg/errors"
)

var (
	ErrInvalidImageName    = errors.New("invalid image name")
	ErrImageNotFound       = errors.New("image not found in registry")
	ErrInvalidTag          = errors.New("invalid tag")
	ErrTagNotFound         = errors.New("tag not found in image")
	ErrRegistryInternal    = errors.New("there was an internal error in the registry")
	ErrInvalidParameter    = errors.New("invalid parameter")
	ErrParameterOutOfRange = errors.New("parameter out of range")
)
