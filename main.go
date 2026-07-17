package main

import (
	"fmt"
	"flag"
	"net/http"
	"os"

	"github.com/decko/wiki-go/internal/config"
	"github.com/decko/wiki-go/internal/logger"
	"github.com/decko/wiki-go/internal/server"
)

func main() {
	configfilepath := flag.String("configfile",
		GetEnvString("CONFIGFILE", config.ConfigFilePath),
		"where to find config.yaml")
	flag.Parse()

	config.ConfigFilePath = *configfilepath

	cfg, err := config.LoadConfig(config.ConfigFilePath)
	if err != nil {
		logger.Fatal("Error loading config: %v", err)
	}

	handler, err := server.NewHandler(cfg)
	if err != nil {
		logger.Fatal("Error initializing server: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.SSL && cfg.Server.SSLCert != "" && cfg.Server.SSLKey != "" {
		logger.Info("HTTPS server starting on %s...", addr)
		if err := http.ListenAndServeTLS(addr, cfg.Server.SSLCert, cfg.Server.SSLKey, handler); err != nil {
			logger.Fatal("HTTPS server error: %v", err)
		}
	} else {
		logger.Info("HTTP server starting on %s...", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			logger.Fatal("HTTP server error: %v", err)
		}
	}
}

func GetEnvString(name, defaultvalue string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultvalue
	}
	return value
}
