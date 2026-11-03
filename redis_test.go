// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func newTestRedisServer(t *testing.T) *redis.Client {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("requires a Redis or Valkey server with Unix socket support")
	}

	if testing.Short() {
		t.Skip("requires a Redis or Valkey server")
	}

	executable := os.Getenv("REDIS_SERVER")
	if executable == "" {
		for _, candidate := range []string{"redis-server", "valkey-server"} {
			if path, err := exec.LookPath(candidate); err == nil {
				executable = path
				break
			}
		}
	}
	if executable == "" {
		t.Fatal("tests require redis-server or valkey-server in PATH, or REDIS_SERVER set to the server executable; use go test -short to skip server-dependent tests")
	}

	dir, err := os.MkdirTemp("", "redis-")
	if err != nil {
		t.Fatalf("create Redis temporary directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	address := filepath.Join(dir, "redis.sock")
	log, err := os.Create(filepath.Join(dir, "redis.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	cmd := exec.Command(
		executable,
		"--port", "0",
		"--unixsocket", address,
		"--unixsocketperm", "700",
		"--save", "",
		"--appendonly", "no",
		"--dir", dir,
		"--daemonize", "no",
	)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatalf("start Redis server: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	client := redis.NewClient(&redis.Options{
		Network:               "unix",
		Addr:                  address,
		MaxRetries:            -1,
		DialTimeout:           100 * time.Millisecond,
		ReadTimeout:           100 * time.Millisecond,
		WriteTimeout:          100 * time.Millisecond,
		ContextTimeoutEnabled: true,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Shutdown(ctx).Err()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	fail := func(format string, args ...any) {
		t.Helper()
		data, _ := os.ReadFile(log.Name())
		t.Logf("Redis server log:\n%s", data)
		t.Fatalf(format, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case err := <-done:
			// Keep the exit result available to cleanup.
			done <- err
			fail("Redis server exited during startup: %v", err)
		default:
		}
		if err := client.Ping(ctx).Err(); err == nil {
			return client
		}
		select {
		case <-ctx.Done():
			fail("timed out waiting for Redis server: %v", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
