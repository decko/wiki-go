package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"wiki-go/internal/auth"
	"wiki-go/internal/config"
	"wiki-go/internal/goldext"
	"wiki-go/internal/handlers"
	"wiki-go/internal/logger"
	"wiki-go/internal/migration"
	"wiki-go/internal/routes"
	"wiki-go/internal/static"
)

func NewHandler(cfg *config.Config) (http.Handler, error) {
	if err := migration.FixBrokenConfig(config.ConfigFilePath); err != nil {
		logger.Warn("Failed to fix broken config: %v", err)
	}

	if err := migration.MigrateUserRoles(config.ConfigFilePath); err != nil {
		return nil, fmt.Errorf("migrate user roles: %w", err)
	}

	goldext.SetWikiTimezone(cfg.Wiki.Timezone)
	logger.Init(cfg.Wiki.LogLevel)

	sessionPath := filepath.Join(cfg.Wiki.RootDir, "temp", "sessions.json")
	if err := auth.InitSessionStore(sessionPath); err != nil {
		logger.Warn("Failed to initialize session store: %v", err)
	}

	if err := handlers.EnsureHomepageExists(cfg); err != nil {
		return nil, fmt.Errorf("create homepage: %w", err)
	}

	if err := static.EnsureStaticAssetsExist(cfg.Wiki.RootDir); err != nil {
		return nil, fmt.Errorf("copy static assets: %w", err)
	}

	handlers.InitHandlers(cfg)

	handler := routes.SetupRoutes(cfg)
	return handler, nil
}
