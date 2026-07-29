// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"fmt"
	"net/url"

	"github.com/ccache/ccache-go-storage-helper"
)

type config struct {
	*storagehelper.Config
	URL *url.URL
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
		warning := fmt.Sprintf("warning: unknown attribute: %s", attribute.Key)
		cfg.Diagnostics = append(cfg.Diagnostics, warning)
	}

	for _, diag := range cfg.Diagnostics {
		logger.Logf("%s", diag)
	}

	return cfg, nil
}
