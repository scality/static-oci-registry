package di

import "github.com/scality/static-oci-registry/pkg/usecase"

func (c *Container) getListTagsUseCase() *usecase.ListTags {
	if c.listTagsUseCase == nil {
		c.listTagsUseCase = usecase.NewListTags(
			c.GetLogger(),
			c.getTagLister(),
		)
	}

	return c.listTagsUseCase
}
