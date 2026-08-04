// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/ccache/ccache-go-storage-helper"
	"github.com/redis/go-redis/v9"
)

const redisPoolTimeout = 24 * time.Hour

type storageClient struct {
	client  *redis.Client
	context context.Context
	logger  *storagehelper.Logger
}

func newStorageClient(cfg *config, logger *storagehelper.Logger) (*storageClient, error) {
	url := *cfg.URL
	if url.Scheme == "redis+unix" {
		if url.Hostname() != "" && url.Hostname() != "localhost" {
			return nil, errors.New("invalid hostname for redis+unix URL")
		}
		url.Scheme = "unix"
	} else if url.Scheme == "redis" || url.Scheme == "rediss" {
		// go-redis ParseURL double-brackets a bare IPv6 address when it adds the
		// default port, so make the address unambiguous before handing it over.
		if url.Port() == "" {
			url.Host = net.JoinHostPort(url.Hostname(), "6379")
		}
	}

	options, err := redis.ParseURL(url.String())
	if err != nil {
		return nil, err
	}

	if options.Username != "" && options.Password == "" {
		// Compatibility with ccache's previous Redis backend: redis://PASSWORD@HOST
		options.Password = options.Username
		options.Username = ""
	}

	// Ccache enforces request timeouts by closing its IPC connection, so do not
	// impose shorter helper-side deadlines on Redis communication.
	//
	// A negative DialTimeout prevents go-redis from applying its default 5s
	// timeout or an internal context deadline. This is safe because redisDialer
	// below uses a net.Dialer with no timeout (the default dialer would interpret
	// a negative timeout as already expired).
	options.DialTimeout = -1
	options.ReadTimeout = -1
	options.WriteTimeout = -1
	// go-redis seems to have no way to disable PoolTimeout, so make it longer
	// than any ccache operation.
	options.PoolTimeout = redisPoolTimeout
	options.Dialer = redisDialer(options.TLSConfig)
	options.ConnMaxIdleTime = 90 * time.Second

	client := redis.NewClient(options)

	sc := &storageClient{
		client:  client,
		context: context.Background(),
		logger:  logger,
	}

	return sc, nil
}

func redisDialer(tlsConfig *tls.Config) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := &net.Dialer{KeepAlive: 5 * time.Minute}
		if tlsConfig == nil {
			return dialer.DialContext(ctx, network, address)
		}

		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsConfig}
		return tlsDialer.DialContext(ctx, network, address)
	}
}

func buildKey(key []byte) string {
	return "ccache:" + hex.EncodeToString(key)
}

func (s *storageClient) Exists(key []byte) (bool, error) {
	keyStr := buildKey(key)

	s.logger.Logf("EXISTS %s", keyStr)
	return s.head(keyStr)
}

func (s *storageClient) Get(key []byte) (io.ReadCloser, int64, bool, error) {
	keyStr := buildKey(key)

	s.logger.Logf("GET %s", keyStr)
	val, err := s.client.Get(s.context, keyStr).Result()
	if err == redis.Nil {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}

	return io.NopCloser(strings.NewReader(val)), int64(len(val)), true, nil
}

func (s *storageClient) Put(key []byte, value io.Reader, size int64, overwrite bool) (bool, error) {
	keyStr := buildKey(key)

	v, err := io.ReadAll(value)
	if err != nil {
		return false, err
	}

	s.logger.Logf("SET %s (%d bytes)", keyStr, size)

	if !overwrite {
		val, err := s.client.SetNX(s.context, keyStr, v, 0).Result()
		if err != nil {
			return false, err
		}
		return val, nil
	}

	err = s.client.Set(s.context, keyStr, v, 0).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *storageClient) Remove(key []byte) (bool, error) {
	keyStr := buildKey(key)

	s.logger.Logf("DEL %s", keyStr)
	val, err := s.client.Del(s.context, keyStr).Result()
	if err != nil {
		return false, err
	}

	return val != 0, nil
}

func (s *storageClient) head(keyStr string) (bool, error) {
	val, err := s.client.Exists(s.context, keyStr).Result()
	if err != nil {
		return false, err
	}

	return val != 0, nil
}
