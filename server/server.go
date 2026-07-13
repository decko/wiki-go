// Package server re-exports the wiki HTTP handler for external consumers.
package server

import (
	"net/http"

	"wiki-go/config"
	internal "wiki-go/internal/server"
)

// NewHandler creates the wiki HTTP handler from the given config.
func NewHandler(cfg *config.Config) (http.Handler, error) {
	return internal.NewHandler(cfg)
}
