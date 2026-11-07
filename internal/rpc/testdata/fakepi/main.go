// Command fakepi speaks just enough of the pi RPC protocol to exercise the
// client. The mode is selected with PI_TUI_RPC_HELPER and each mode covers one
// failure or framing scenario.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const modeEnv = "PI_TUI_RPC_HELPER"

func main() {
	mode := os.Getenv(modeEnv)
	if mode == "" {
		fmt.Fprintln(os.Stderr, "fakepi: PI_TUI_RPC_HELPER is required")
		os.Exit(2)
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 8<<20)
	out := bufio.NewWriter(os.Stdout)

	respond := func(command map[string]any) {
		payload := map[string]any{
			"type":    "response",
			"id":      command["id"],
			"command": command["type"],
			"success": true,
			"data":    map[string]any{"echo": command["type"]},
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = out.Write(append(encoded, '\n'))
		_ = out.Flush()
	}

	readCommand := func() (map[string]any, bool) {
		for in.Scan() {
			var command map[string]any
			if err := json.Unmarshal(in.Bytes(), &command); err != nil {
				continue
			}
			return command, true
		}
		return nil, false
	}

	switch mode {
	case "echo":
		fmt.Fprintln(out, `{"type":"agent_start"}`)
		_ = out.Flush()
		for {
			command, ok := readCommand()
			if !ok {
				return
			}
			respond(command)
		}

	case "outoforder":
		// Answer the second request before the first one.
		var commands []map[string]any
		for len(commands) < 2 {
			command, ok := readCommand()
			if !ok {
				return
			}
			commands = append(commands, command)
		}
		for i := len(commands) - 1; i >= 0; i-- {
			respond(commands[i])
		}
		for in.Scan() {
		}

	case "noisy":
		fmt.Fprint(out, "not json at all\n")
		fmt.Fprint(out, `{"type":"future_record","payload":42}`+"\n")
		fmt.Fprint(out, `{"type":"agent_start"}`+"\r\n")
		_ = out.Flush()
		for {
			command, ok := readCommand()
			if !ok {
				return
			}
			respond(command)
		}

	case "bigline":
		payload, err := json.Marshal(map[string]any{
			"type":  "message_update",
			"usage": map[string]any{"input": 1, "output": 1, "totalTokens": 2},
			"assistantMessageEvent": map[string]any{
				"type":         "text_delta",
				"contentIndex": 0,
				"delta":        strings.Repeat("x", 1<<20),
			},
		})
		if err != nil {
			os.Exit(1)
		}
		fmt.Fprintf(out, "%s\n", payload)
		_ = out.Flush()
		for {
			command, ok := readCommand()
			if !ok {
				return
			}
			respond(command)
		}

	case "crash":
		fmt.Fprintln(os.Stderr, "boom: model not configured")
		os.Exit(3)

	case "stubborn":
		// Ignore stdin EOF and never answer, forcing the client to escalate.
		for {
			time.Sleep(50 * time.Millisecond)
		}

	case "silent":
		for in.Scan() {
		}

	default:
		fmt.Fprintf(os.Stderr, "fakepi: unknown mode %q\n", mode)
		os.Exit(2)
	}
}
