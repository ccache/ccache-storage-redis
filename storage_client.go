// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"context"
	"encoding/hex"
	"io"
	"net/url"
	"path"
	"strconv"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const httpTransportBufferSize = 64 << 10

type storageClient struct {
	client        *redis.Client
	context       context.Context
	timeout       time.Duration
	baseURL       *url.URL
	prefix        string
	bearerToken   string
	basicAuthUser string
	basicAuthPass string
	logger        *logger
}

func newStorageClient(cfg *config, logger *logger) (*storageClient, error) {
	username := cfg.URL.User.Username()
	password, _ := cfg.URL.User.Password()
	// ccache sends password only as username
	if username != "" && password == "" {
		password = username
		username = ""
	}
	var network string
	var addr string
	var dbstr string
	switch cfg.URL.Scheme {
	case "redis":
		network = "tcp"
		addr = cfg.URL.Host
		dbstr = path.Base(cfg.URL.Path)
	case "redis+unix":
		network = "unix"
		addr = cfg.URL.Path
		dbstr = cfg.URL.Query().Get("db")
	}
	db := 0
	if dbstr != "." && dbstr != "" {
		i, err := strconv.Atoi(dbstr)
		if err != nil {
			return nil, err
		}
		db = i
	}
	client := redis.NewClient(&redis.Options{
		Network:         network,
		Username:        username,
		Password:        password,
		Addr:            addr,
		DB:              db,
		ConnMaxIdleTime: 90 * time.Second,
	})

	sc := &storageClient{
		client:      client,
		context:     context.Background(),
		timeout:     10 * time.Second,
		baseURL:     cfg.URL,
		prefix:      "ccache",
		bearerToken: cfg.BearerToken,
		logger:      logger,
	}

	if cfg.UseNetrc {
		netrcPath := cfg.NetrcFile
		if netrcPath == "" {
			netrcPath = defaultNetrcPath()
		}
		if netrcPath != "" {
			requestedLogin := ""
			if cfg.URL.User != nil {
				requestedLogin = cfg.URL.User.Username()
			}

			login, password, err := findNetrcCredentials(netrcPath, cfg.URL.Hostname(), requestedLogin)
			if err != nil {
				if !os.IsNotExist(err) {
					logger.logf("Warning: could not read netrc file %q: %v", netrcPath, err)
				}
			} else {
				sc.basicAuthUser = login
				sc.basicAuthPass = password
			}
		}
	}

	return sc, nil
}

func (s *storageClient) keyToPath(key []byte) string {
	return hex.EncodeToString(key)
}

func (s *storageClient) buildURL(key []byte) (string, error) {
	path := s.keyToPath(key)
	return s.prefix + ":" + path, nil
}

func (s *storageClient) exists(key []byte) (bool, error) {
	urlStr, err := s.buildURL(key)
	if err != nil {
		return false, err
	}

	s.logger.logf("EXISTS %s", urlStr)
	return s.head(urlStr)
}

func (s *storageClient) get(key []byte) (io.ReadCloser, int64, bool, error) {
	urlStr, err := s.buildURL(key)
	if err != nil {
		return nil, 0, false, err
	}

	s.logger.logf("GET %s", urlStr)
	ctx, cancel := context.WithTimeout(s.context, s.timeout)
	defer cancel()
	val, err := s.client.Get(ctx, urlStr).Result()
	if err == redis.Nil {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}

	return io.NopCloser(strings.NewReader(val)), int64(len(val)), true, nil
}

func (s *storageClient) put(key []byte, value io.Reader, size int64, overwrite bool) (bool, error) {
	urlStr, err := s.buildURL(key)
	if err != nil {
		return false, err
	}

	if !overwrite {
		exists, err := s.head(urlStr)
		if err != nil {
			return false, err
		}
		if exists {
			return false, nil
		}
	}

	s.logger.logf("SET %s (%d bytes)", urlStr, size)
	ctx, cancel := context.WithTimeout(s.context, s.timeout)
	defer cancel()
	err = s.client.Set(ctx, urlStr, value, 0).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *storageClient) remove(key []byte) (bool, error) {
	urlStr, err := s.buildURL(key)
	if err != nil {
		return false, err
	}

	s.logger.logf("DEL %s", urlStr)
	ctx, cancel := context.WithTimeout(s.context, s.timeout)
	defer cancel()
	val, err := s.client.Del(ctx, urlStr).Result()
	if err != nil {
		return false, err
	}

	return val != 0, nil
}

func (s *storageClient) head(urlStr string) (bool, error) {
	ctx, cancel := context.WithTimeout(s.context, s.timeout)
	defer cancel()
	val, err := s.client.Exists(ctx, urlStr).Result()
	if err != nil {
		return false, err
	}

	return val != 0, nil
}
