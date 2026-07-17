package handlers

import (
	"github.com/decko/wiki-go/internal/config"
	"github.com/decko/wiki-go/internal/i18n"
	"github.com/decko/wiki-go/internal/logger"
)

var cfg *config.Config

// InitHandlers initializes the handlers with the given configuration
func InitHandlers(config *config.Config) {
	cfg = config

	// Initialize i18n package
	if err := i18n.Initialize(cfg); err != nil {
		logger.Warn("Failed to initialize i18n package: %v", err)
	}

	// Initialise IP-based ban list for login attempts
	InitLoginBan(cfg)

	// Routes are now managed in the routes package
}

// We don't need this anymore since main.go handles the routing
// func handlePage(w http.ResponseWriter, r *http.Request) {
//     PageHandler(w, r, cfg)
// }
