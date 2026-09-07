// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/ccache/ccache-go-storage-helper"
)

type config struct {
	*storagehelper.Config
	URL                *url.URL
	ConnectionPoolSize int
}

func parseConfig(logger *storagehelper.Logger) (*config, error) {
	baseConfig, err := storagehelper.ParseConfig(logger)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(baseConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_URL: %w", err)
	}
	cfg := &config{Config: baseConfig, URL: parsedURL}
	logger.Logf("URL: %s", cfg.URL)

	for _, attribute := range cfg.Attributes {
		key := attribute.Key
		value := attribute.Value

		switch key {
		case "connection-pool-size":
			size, err := strconv.Atoi(value)
			if err != nil {
				cfg.Diagnostics = append(cfg.Diagnostics, fmt.Sprintf("error: invalid connection pool size %q: %v", value, err))
			} else if size <= 0 {
				cfg.Diagnostics = append(cfg.Diagnostics, fmt.Sprintf("error: invalid connection pool size %q: must be positive", value))
			} else {
				cfg.ConnectionPoolSize = size
			}
		default:
			warning := fmt.Sprintf("warning: unknown attribute: %s", attribute.Key)
			cfg.Diagnostics = append(cfg.Diagnostics, warning)
		}
	}

	for _, diag := range cfg.Diagnostics {
		logger.Logf("%s", diag)
	}

	return cfg, nil
}
