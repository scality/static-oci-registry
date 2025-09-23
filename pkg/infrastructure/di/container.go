package di

import "net/http"

type Container struct {
	httpServer *http.Server
}

func NewContainer() *Container {
	return &Container{}
}
