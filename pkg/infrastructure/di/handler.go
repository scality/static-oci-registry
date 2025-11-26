package di

import "github.com/scality/static-oci-registry/pkg/presentation/http/handler"

func (c *Container) getListTagsHandler() *handler.ListTags {
	if c.listTagsHandler == nil {
		c.listTagsHandler = handler.NewListTags(c.getListTagsUseCase(), c.GetLogger())
	}

	return c.listTagsHandler
}
