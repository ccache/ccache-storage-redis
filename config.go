// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"time"
)

type layout string

const (
	layoutBazel   layout = "bazel"
	layoutFlat    layout = "flat"
	layoutSubdirs layout = "subdirs"
)

type config struct {
	IPCEndpoint string
	URL         *url.URL
	IdleTimeout time.Duration
	Diagnostics []string
	BearerToken string
	UseNetrc    bool
	NetrcFile   string
}

func parseConfig(logger *logger) (*config, error) {
	ipcEndpoint := os.Getenv("CRSH_IPC_ENDPOINT")
	if runtime.GOOS == "windows" {
		ipcEndpoint = `\\.\pipe\` + ipcEndpoint
	}
	logger.logf("IPC endpoint: %s", ipcEndpoint)

	cfg := &config{
		IPCEndpoint: ipcEndpoint,
	}

	urlStr := os.Getenv("CRSH_URL")
	if urlStr == "" {
		return nil, fmt.Errorf("CRSH_URL not set")
	}
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_URL: %w", err)
	}
	cfg.URL = parsedURL
	logger.logf("URL: %s", cfg.URL)

	idleTimeout := os.Getenv("CRSH_IDLE_TIMEOUT")
	if idleTimeout == "" {
		idleTimeout = "0"
	}
	timeoutSecs, err := strconv.Atoi(idleTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_IDLE_TIMEOUT: %w", err)
	}
	cfg.IdleTimeout = time.Duration(timeoutSecs) * time.Second
	logger.logf("Idle timeout: %s", cfg.IdleTimeout)

	numAttr := os.Getenv("CRSH_NUM_ATTR")
	if numAttr == "" {
		numAttr = "0"
	}
	n, err := strconv.Atoi(numAttr)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_NUM_ATTR: %w", err)
	}
	for i := 0; i < n; i++ {
		key := os.Getenv(fmt.Sprintf("CRSH_ATTR_KEY_%d", i))
		value := os.Getenv(fmt.Sprintf("CRSH_ATTR_VALUE_%d", i))
		logger.logf("Attribute: %s=%s", key, value)

		switch key {
		case "bearer-token":
			cfg.BearerToken = value
		case "netrc-file":
			cfg.NetrcFile = value
			cfg.UseNetrc = true
		case "use-netrc":
			cfg.UseNetrc = value == "true"
		default:
			cfg.Diagnostics = append(cfg.Diagnostics, fmt.Sprintf("warning: unknown attribute: %s", key))
		}
	}

	for _, diag := range cfg.Diagnostics {
		logger.logf("%s", diag)
	}

	return cfg, nil
}
