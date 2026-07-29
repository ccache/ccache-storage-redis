// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"io"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ccache/ccache-go-storage-helper"
)

func TestNewStorageClientURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantNetwork  string
		wantAddr     string
		wantDB       int
		wantUsername string
		wantPassword string
		wantTLS      bool
		wantServer   string
	}{
		{
			name:        "Redis default port",
			url:         "redis://cache.example.com",
			wantNetwork: "tcp",
			wantAddr:    "cache.example.com:6379",
		},
		{
			name:        "Redis explicit port and database",
			url:         "redis://cache.example.com:6380/3",
			wantNetwork: "tcp",
			wantAddr:    "cache.example.com:6380",
			wantDB:      3,
		},
		{
			name:        "Redis IPv6 default port",
			url:         "redis://[2001:db8::1]",
			wantNetwork: "tcp",
			wantAddr:    "[2001:db8::1]:6379",
		},
		{
			name:        "Redis IPv6 explicit port",
			url:         "redis://[2001:db8::1]:6380/3",
			wantNetwork: "tcp",
			wantAddr:    "[2001:db8::1]:6380",
			wantDB:      3,
		},
		{
			name:         "Redis username and password",
			url:          "redis://alice:secret@cache.example.com",
			wantNetwork:  "tcp",
			wantAddr:     "cache.example.com:6379",
			wantUsername: "alice",
			wantPassword: "secret",
		},
		{
			name:         "Redis password-only compatibility",
			url:          "redis://secret@cache.example.com",
			wantNetwork:  "tcp",
			wantAddr:     "cache.example.com:6379",
			wantPassword: "secret",
		},
		{
			name:        "Redis TLS",
			url:         "rediss://cache.example.com/2",
			wantNetwork: "tcp",
			wantAddr:    "cache.example.com:6379",
			wantDB:      2,
			wantTLS:     true,
			wantServer:  "cache.example.com",
		},
		{
			name:        "Redis Unix socket",
			url:         "redis+unix:/run/redis.sock?db=3",
			wantNetwork: "unix",
			wantAddr:    "/run/redis.sock",
			wantDB:      3,
		},
		{
			name:         "Redis Unix socket with password",
			url:          "redis+unix://secret@localhost/run/redis.sock?db=3",
			wantNetwork:  "unix",
			wantAddr:     "/run/redis.sock",
			wantDB:       3,
			wantPassword: "secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("parse URL: %v", err)
			}

			client, err := newStorageClient(&config{URL: parsedURL}, storagehelper.NewLogger(""))
			if err != nil {
				t.Fatalf("newStorageClient returned error: %v", err)
			}
			defer client.client.Close()

			options := client.client.Options()
			if options.Network != tt.wantNetwork {
				t.Errorf("Network = %q, want %q", options.Network, tt.wantNetwork)
			}
			if options.Addr != tt.wantAddr {
				t.Errorf("Addr = %q, want %q", options.Addr, tt.wantAddr)
			}
			if options.DB != tt.wantDB {
				t.Errorf("DB = %d, want %d", options.DB, tt.wantDB)
			}
			if options.Username != tt.wantUsername {
				t.Errorf("Username = %q, want %q", options.Username, tt.wantUsername)
			}
			if options.Password != tt.wantPassword {
				t.Errorf("Password = %q, want %q", options.Password, tt.wantPassword)
			}
			if (options.TLSConfig != nil) != tt.wantTLS {
				t.Errorf("TLSConfig present = %v, want %v", options.TLSConfig != nil, tt.wantTLS)
			}
			if tt.wantTLS && options.TLSConfig.ServerName != tt.wantServer {
				t.Errorf("TLSConfig.ServerName = %q, want %q", options.TLSConfig.ServerName, tt.wantServer)
			}
		})
	}
}

func TestNewStorageClientRejectsInvalidURL(t *testing.T) {
	tests := []string{
		"http://cache.example.com",
		"redis://cache.example.com/not-a-database",
		"redis+unix://cache.example.com/run/redis.sock",
	}

	for _, urlString := range tests {
		t.Run(urlString, func(t *testing.T) {
			parsedURL, err := url.Parse(urlString)
			if err != nil {
				t.Fatalf("parse URL: %v", err)
			}

			if _, err := newStorageClient(&config{URL: parsedURL}, storagehelper.NewLogger("")); err == nil {
				t.Fatal("newStorageClient returned nil error")
			}
		})
	}
}

func TestNewStorageClientDisablesRedisTimeouts(t *testing.T) {
	parsedURL, err := url.Parse("rediss://cache.example.com")
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}

	client, err := newStorageClient(&config{URL: parsedURL}, storagehelper.NewLogger(""))
	if err != nil {
		t.Fatalf("newStorageClient returned error: %v", err)
	}
	defer client.client.Close()

	options := client.client.Options()
	if options.DialTimeout != -1 {
		t.Errorf("DialTimeout = %v, want -1", options.DialTimeout)
	}
	// go-redis normalizes -1 to zero after interpreting it as disabled.
	if options.ReadTimeout != 0 {
		t.Errorf("ReadTimeout = %v, want 0", options.ReadTimeout)
	}
	if options.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0", options.WriteTimeout)
	}
	if options.PoolTimeout != redisPoolTimeout {
		t.Errorf("PoolTimeout = %v, want %v", options.PoolTimeout, redisPoolTimeout)
	}
	if options.Dialer == nil {
		t.Error("Dialer is nil")
	}
	if redisPoolTimeout <= time.Hour {
		t.Errorf("redisPoolTimeout = %v, want more than one hour", redisPoolTimeout)
	}
}

func TestStorageClientAllowsConcurrentRequests(t *testing.T) {
	server := miniredis.RunT(t)

	url, err := url.Parse("redis://" + server.Addr())
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	client, err := newStorageClient(&config{URL: url}, storagehelper.NewLogger(""))
	if err != nil {
		t.Fatalf("newStorageClient returned error: %v", err)
	}
	defer client.client.Close()

	if err = server.Set("ccache:01", "1"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err = server.Set("ccache:02", "2"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	start := make(chan struct{})
	var ready sync.WaitGroup
	var wg sync.WaitGroup
	for _, key := range [][]byte{{0x01}, {0x02}} {
		ready.Add(1)
		wg.Add(1)
		go func(key []byte) {
			defer wg.Done()
			ready.Done()
			<-start

			body, _, found, err := client.Get(key)
			if body != nil {
				defer body.Close()
			}
			if err != nil {
				t.Errorf("get(%x) returned error: %v", key, err)
			} else if !found {
				t.Errorf("get(%x) returned found=false, want true", key)
			} else if _, err := io.Copy(io.Discard, body); err != nil {
				t.Errorf("drain get(%x) body: %v", key, err)
			}
		}(key)
	}

	ready.Wait()
	close(start)
	wg.Wait()
}
