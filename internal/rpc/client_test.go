package rpc

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// helperEnv selects the fake pi mode.
const helperEnv = "PI_TUI_RPC_HELPER"

// fakePiPath is a real child process built from testdata/fakepi. Using the test
// binary itself does not work: modern Go testing captures os.Stdout, which
// delays protocol writes until the process exits.
var fakePiPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pie-tui-fakepi")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fakepi: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	fakePiPath = filepath.Join(dir, "fakepi")
	build := exec.Command("go", "build", "-o", fakePiPath, "./testdata/fakepi")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fakepi: build failed: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func helperClient(t *testing.T, mode string) *Client {
	t.Helper()
	client, err := Start(context.Background(), Options{
		Bin:            fakePiPath,
		Args:           []string{"--mode", "rpc"},
		Env:            append(os.Environ(), helperEnv+"="+mode),
		ShutdownGrace:  500 * time.Millisecond,
		TerminateGrace: 500 * time.Millisecond,
		Logf:           t.Logf,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestClientCorrelatesResponses(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "echo")

	// The first event arrives without any command being sent.
	select {
	case event := <-client.Events():
		if _, ok := event.(*AgentStartEvent); !ok {
			t.Fatalf("first event = %T, want *AgentStartEvent", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for agent_start")
	}

	response, err := client.DoOK(context.Background(), NewCommand("get_state"))
	if err != nil {
		t.Fatalf("DoOK: %v", err)
	}
	if response.ID == "" || response.Command != "get_state" {
		t.Fatalf("unexpected response: %+v", response)
	}

	var data map[string]any
	if err := response.DecodeData(&data); err != nil || data["echo"] != "get_state" {
		t.Fatalf("unexpected data: %+v (err %v)", data, err)
	}
}

func TestClientMatchesOutOfOrderResponses(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "outoforder")

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, kind := range []string{"first", "second"} {
		kind := kind
		wg.Add(1)
		go func() {
			defer wg.Done()
			command := NewCommand(kind)
			response, err := client.DoOK(context.Background(), command)
			if err != nil {
				errs <- fmt.Errorf("%s: %w", kind, err)
				return
			}
			if response.Command != kind || response.ID != command["id"] {
				errs <- fmt.Errorf("%s: mismatched response %+v", kind, response)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestClientToleratesGarbageRecords(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "noisy")

	seen := map[string]bool{}
	timeout := time.After(5 * time.Second)
	for len(seen) < 2 {
		select {
		case event, ok := <-client.Events():
			if !ok {
				t.Fatal("event stream closed early")
			}
			seen[event.Kind()] = true
		case <-timeout:
			t.Fatalf("timed out, saw %v", seen)
		}
	}
	if !seen["future_record"] || !seen["agent_start"] {
		t.Fatalf("missing events: %v", seen)
	}

	if _, err := client.DoOK(context.Background(), NewCommand("get_state")); err != nil {
		t.Fatalf("client unusable after garbage: %v", err)
	}
}

func TestClientReadsLargeRecords(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "bigline")

	select {
	case event := <-client.Events():
		update, ok := event.(*MessageUpdateEvent)
		if !ok {
			t.Fatalf("event = %T", event)
		}
		if len(update.Assistant.Delta) != 1<<20 {
			t.Fatalf("delta length = %d", len(update.Assistant.Delta))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for large record")
	}
}

func TestClientReportsChildExit(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "crash")

	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("child exit was not observed")
	}

	if err := client.Err(); err == nil {
		t.Fatal("expected an exit error")
	}
	if tail := strings.Join(client.StderrTail(), "\n"); !strings.Contains(tail, "model not configured") {
		t.Fatalf("stderr tail = %q", tail)
	}

	if _, err := client.Do(context.Background(), NewCommand("get_state")); err == nil {
		t.Fatal("expected error after child exit")
	}
}

func TestClientRejectsOversizedRecords(t *testing.T) {
	t.Parallel()
	var (
		logs  []string
		logMu sync.Mutex
	)
	client, err := Start(context.Background(), Options{
		Bin:          fakePiPath,
		Env:          append(os.Environ(), helperEnv+"=bigline"),
		MaxLineBytes: 4096,
		Logf: func(format string, args ...any) {
			logMu.Lock()
			logs = append(logs, fmt.Sprintf(format, args...))
			logMu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = client.Close() }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		logMu.Lock()
		found := false
		for _, line := range logs {
			if strings.Contains(line, "oversized") {
				found = true
			}
		}
		logMu.Unlock()
		if found {
			// The stream must stay in sync after a dropped record.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := client.DoOK(ctx, NewCommand("ping")); err != nil {
				t.Fatalf("client unusable after dropped record: %v", err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for oversized record log")
}

func TestClientRequestTimeout(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "silent")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := client.Do(ctx, NewCommand("get_state")); err == nil {
		t.Fatal("expected timeout error")
	} else if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientCloseIsGraceful(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "echo")

	start := time.Now()
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("graceful close took %v", elapsed)
	}

	select {
	case <-client.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done did not close after Close")
	}

	// Close must be idempotent.
	if err := client.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestClientCloseKillsStubbornChild(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "stubborn")

	start := time.Now()
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("escalating close took %v", elapsed)
	}

	select {
	case <-client.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done did not close after kill")
	}
}

func TestClientRejectsCommandWithoutType(t *testing.T) {
	t.Parallel()
	client := helperClient(t, "echo")

	if _, err := client.Do(context.Background(), Command{"id": "x"}); err == nil {
		t.Fatal("expected error for command without type")
	}
}

func TestReadLineSplitsOnlyOnLF(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReaderSize(strings.NewReader("a\u2028b\nsecond\r\nthird"), 8)
	first, err := readLine(reader, 1024)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(first) != "a\u2028b" {
		t.Fatalf("first = %q", first)
	}

	second, err := readLine(reader, 1024)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(second) != "second" {
		t.Fatalf("second = %q", second)
	}

	third, err := readLine(reader, 1024)
	if err == nil {
		t.Fatal("expected EOF on the trailing record")
	}
	if string(third) != "third" {
		t.Fatalf("third = %q", third)
	}
}

func TestReadLineRejectsOversizedRecordAndResyncs(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReaderSize(strings.NewReader(strings.Repeat("a", 5000)+"\nok\n"), 16)
	if _, err := readLine(reader, 1024); err == nil {
		t.Fatal("expected ErrLineTooLong")
	}
	line, err := readLine(reader, 1024)
	if err != nil || string(line) != "ok" {
		t.Fatalf("resync line = %q (err %v)", line, err)
	}
}

func TestTailBufferKeepsLastLines(t *testing.T) {
	t.Parallel()

	tail := newTailBuffer(2)
	if _, err := tail.Write([]byte("one\ntwo\nthree")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// "three" has no newline yet, so it stays in the carry buffer.
	if got := tail.Lines(); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("lines = %v", got)
	}

	if _, err := tail.Write([]byte(" four\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := tail.Lines(); len(got) != 2 || got[0] != "two" || got[1] != "three four" {
		t.Fatalf("lines = %v", got)
	}
}
