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
			Handler:           c.getHTTPRouter(),
			ReadHeaderTimeout: timeoutDurationInSeconds * time.Second,
			TLSConfig:         c.getTLSConfig(),
		}
	}

	return c.httpServer
}
