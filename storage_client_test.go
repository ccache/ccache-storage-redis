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
)

func TestStorageClientAllowsConcurrentRequests(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})

	server := miniredis.RunT(t)

	baseURL, err := url.Parse("redis://" + server.Addr())
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	client, err := newStorageClient(&config{
		URL: baseURL,
	}, newLogger(""))
	if err != nil {
		t.Fatalf("newStorageClient returned error: %v", err)
	}

	if err = server.Set("ccache:01", "1"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err = server.Set("ccache:02", "2"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	var wg sync.WaitGroup
	for _, key := range [][]byte{{0x01}, {0x02}} {
		wg.Add(1)
		go func(key []byte) {
			defer wg.Done()
			body, _, found, err := client.get(key)
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

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			close(release)
			wg.Wait()
			t.Fatal("timed out waiting for concurrent requests to reach the server")
		}
	}

	close(release)
	wg.Wait()
}
