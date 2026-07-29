// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

//go:build !windows

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ccache/ccache-go-storage-helper"
)

const testHelperEnv = "CRSH_TEST_HELPER"

const (
	protocolVersion  = storagehelper.ProtocolVersion
	capGetPutRemove  = storagehelper.CapabilityGetPutRemove
	capInfo          = storagehelper.CapabilityInfo
	capExists        = storagehelper.CapabilityExists
	requestGet       = storagehelper.RequestGet
	requestPut       = storagehelper.RequestPut
	requestRemove    = storagehelper.RequestRemove
	requestStop      = storagehelper.RequestStop
	requestInfo      = storagehelper.RequestInfo
	requestExists    = storagehelper.RequestExists
	responseOK       = storagehelper.ResponseOK
	responseNoop     = storagehelper.ResponseNoop
	responseErr      = storagehelper.ResponseErr
	putFlagOverwrite = storagehelper.PutFlagOverwrite
)

func TestMain(m *testing.M) {
	if os.Getenv(testHelperEnv) == "1" {
		main()
		os.Exit(0)
	}

	os.Exit(m.Run())
}

type helperProcess struct {
	t          *testing.T
	socketPath string
	logPath    string
	cmd        *exec.Cmd
	conn       net.Conn
}

func newHelperProcess(t *testing.T, redisURL string, attrs [][2]string) *helperProcess {
	t.Helper()

	tmpDir := t.TempDir()
	h := &helperProcess{
		t:          t,
		socketPath: filepath.Join(tmpDir, "storagehelper.sock"),
		logPath:    filepath.Join(tmpDir, "storagehelper.log"),
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	env := append(os.Environ(),
		testHelperEnv+"=1",
		"CRSH_IPC_ENDPOINT="+h.socketPath,
		"CRSH_URL="+redisURL,
		"CRSH_IDLE_TIMEOUT=30",
		fmt.Sprintf("CRSH_NUM_ATTR=%d", len(attrs)),
		"CRSH_LOGFILE="+h.logPath,
	)
	for i, attr := range attrs {
		env = append(env,
			fmt.Sprintf("CRSH_ATTR_KEY_%d=%s", i, attr[0]),
			fmt.Sprintf("CRSH_ATTR_VALUE_%d=%s", i, attr[1]),
		)
	}

	h.cmd = exec.Command(exe)
	h.cmd.Env = env
	if err := h.cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(h.stop)

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial("unix", h.socketPath)
		if err == nil {
			h.conn = conn
			break
		}
		if time.Now().After(deadline) {
			h.stop()
			t.Fatalf("timed out connecting to IPC socket: %v; helper log:\n%s", err, h.readLog())
		}
		time.Sleep(10 * time.Millisecond)
	}

	greeting := make([]byte, 5)
	h.readFull(greeting)
	wantGreeting := []byte{protocolVersion, 3, capGetPutRemove, capInfo, capExists}
	if !bytes.Equal(greeting, wantGreeting) {
		t.Fatalf("greeting = %v, want %v", greeting, wantGreeting)
	}

	return h
}

func (h *helperProcess) stop() {
	if h.conn != nil {
		_ = h.conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = h.conn.Write([]byte{requestStop})
		var status [1]byte
		_, _ = io.ReadFull(h.conn, status[:])
		_ = h.conn.Close()
		h.conn = nil
	}

	if h.cmd != nil && h.cmd.Process != nil {
		done := make(chan error, 1)
		go func() { done <- h.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = h.cmd.Process.Kill()
			<-done
		}
		h.cmd = nil
	}
}

func (h *helperProcess) info() (string, []string) {
	h.write([]byte{requestInfo})

	identity := h.readMsg()
	diagnostics := make([]string, h.readByte())
	for i := range diagnostics {
		diagnostics[i] = h.readMsg()
	}
	return identity, diagnostics
}

func (h *helperProcess) get(key []byte) (byte, []byte) {
	h.write(append([]byte{requestGet, byte(len(key))}, key...))

	status := h.readByte()
	switch status {
	case responseOK:
		var length [8]byte
		h.readFull(length[:])
		value := make([]byte, binary.NativeEndian.Uint64(length[:]))
		h.readFull(value)
		return status, value
	case responseNoop:
		return status, nil
	case responseErr:
		return status, []byte(h.readMsg())
	default:
		h.t.Fatalf("unexpected GET status: %d", status)
		return 0, nil
	}
}

func (h *helperProcess) put(key, value []byte, overwrite bool) (byte, []byte) {
	flags := byte(0)
	if overwrite {
		flags = putFlagOverwrite
	}
	var length [8]byte
	binary.NativeEndian.PutUint64(length[:], uint64(len(value)))

	request := append([]byte{requestPut, byte(len(key))}, key...)
	request = append(request, flags)
	request = append(request, length[:]...)
	request = append(request, value...)
	h.write(request)

	status := h.readByte()
	if status == responseErr {
		return status, []byte(h.readMsg())
	}
	if status != responseOK && status != responseNoop {
		h.t.Fatalf("unexpected PUT status: %d", status)
	}
	return status, nil
}

func (h *helperProcess) exists(key []byte) (byte, bool, []byte) {
	h.write(append([]byte{requestExists, byte(len(key))}, key...))

	status := h.readByte()
	if status == responseErr {
		return status, false, []byte(h.readMsg())
	}
	if status != responseOK {
		h.t.Fatalf("unexpected EXISTS status: %d", status)
	}
	return status, h.readByte() != 0, nil
}

func (h *helperProcess) remove(key []byte) (byte, []byte) {
	h.write(append([]byte{requestRemove, byte(len(key))}, key...))

	status := h.readByte()
	if status == responseErr {
		return status, []byte(h.readMsg())
	}
	if status != responseOK && status != responseNoop {
		h.t.Fatalf("unexpected REMOVE status: %d", status)
	}
	return status, nil
}

func (h *helperProcess) write(data []byte) {
	h.t.Helper()
	for len(data) > 0 {
		n, err := h.conn.Write(data)
		if err != nil {
			h.t.Fatalf("write IPC request: %v", err)
		}
		data = data[n:]
	}
}

func (h *helperProcess) readFull(data []byte) {
	h.t.Helper()
	if _, err := io.ReadFull(h.conn, data); err != nil {
		h.t.Fatalf("read IPC response: %v; helper log:\n%s", err, h.readLog())
	}
}

func (h *helperProcess) readByte() byte {
	var value [1]byte
	h.readFull(value[:])
	return value[0]
}

func (h *helperProcess) readMsg() string {
	value := make([]byte, h.readByte())
	h.readFull(value)
	return string(value)
}

func (h *helperProcess) readLog() string {
	data, err := os.ReadFile(h.logPath)
	if err != nil {
		return fmt.Sprintf("(could not read log: %v)", err)
	}
	return string(data)
}

func TestIntegrationInfo(t *testing.T) {
	server := miniredis.RunT(t)
	h := newHelperProcess(t, "redis://"+server.Addr(), [][2]string{{"unknown", "value"}})

	identity, diagnostics := h.info()
	if identity != "ccache-storage-redis "+version {
		t.Errorf("identity = %q, want %q", identity, "ccache-storage-redis "+version)
	}
	wantDiagnostics := []string{"warning: unknown attribute: unknown"}
	if len(diagnostics) != len(wantDiagnostics) {
		t.Fatalf("diagnostics = %q, want %q", diagnostics, wantDiagnostics)
	}
	for i, want := range wantDiagnostics {
		if diagnostics[i] != want {
			t.Errorf("diagnostics[%d] = %q, want %q", i, diagnostics[i], want)
		}
	}
}

func TestIntegrationObjectLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	h := newHelperProcess(t, "redis://"+server.Addr(), nil)

	key := []byte{0x01, 0x23}
	value := []byte("first value")
	if status, message := h.put(key, value, true); status != responseOK {
		t.Fatalf("PUT status = %d, want OK (message %q)", status, message)
	}
	if storedValue, err := server.Get("ccache:0123"); err != nil || storedValue != string(value) {
		t.Fatalf("Redis value = %q, %v; want %q, nil", storedValue, err, value)
	}

	if status, exists, message := h.exists(key); status != responseOK || !exists {
		t.Fatalf("EXISTS = (%d, %v, %q), want (OK, true, nil)", status, exists, message)
	}
	if status, got := h.get(key); status != responseOK || !bytes.Equal(got, value) {
		t.Fatalf("GET = (%d, %q), want (OK, %q)", status, got, value)
	}

	if status, message := h.put(key, []byte("replacement"), false); status != responseNoop {
		t.Fatalf("PUT without overwrite = (%d, %q), want (NOOP, nil)", status, message)
	}
	if storedValue, err := server.Get("ccache:0123"); err != nil || storedValue != string(value) {
		t.Fatalf("Redis value after no-op = %q, %v; want %q, nil", storedValue, err, value)
	}

	if status, message := h.remove(key); status != responseOK {
		t.Fatalf("REMOVE status = (%d, %q), want (OK, nil)", status, message)
	}
	if status, exists, message := h.exists(key); status != responseOK || exists {
		t.Fatalf("EXISTS after remove = (%d, %v, %q), want (OK, false, nil)", status, exists, message)
	}
	if status, got := h.get(key); status != responseNoop || got != nil {
		t.Fatalf("GET after remove = (%d, %q), want (NOOP, nil)", status, got)
	}
	if status, message := h.remove(key); status != responseNoop {
		t.Fatalf("REMOVE missing key = (%d, %q), want (NOOP, nil)", status, message)
	}
}
