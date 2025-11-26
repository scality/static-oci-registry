package di

import (
	"net/http"
	"time"
)

const timeoutDurationInSeconds = 5

func (c *Container) GetHTTPServer() *http.Server {
	if c.httpServer == nil {
		c.httpServer = &http.Server{
			Addr:              c.config.HTTP.Addr,
			Handler:           c.getRouter(),
			ReadHeaderTimeout: timeoutDurationInSeconds * time.Second,
		}
	}

	return c.httpServer
}
