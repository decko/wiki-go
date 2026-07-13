// Package config re-exports wiki-go configuration types for external consumers.
package config

import internal "wiki-go/internal/config"

// Config represents the wiki server configuration.
type Config = internal.Config

// ConfigFilePath is the path to the configuration file.
var ConfigFilePath = &internal.ConfigFilePath

// LoadConfig loads the configuration from a YAML file.
var LoadConfig = internal.LoadConfig
