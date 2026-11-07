package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lihaozhe/pie-tui/internal/rpc"
	"github.com/lihaozhe/pie-tui/internal/session"
)

// handleEvent folds one protocol event into the UI state.
func (m *Model) handleEvent(event rpc.Event) (tea.Model, tea.Cmd) {
	switch ev := event.(type) {
	case *rpc.AgentStartEvent:
		m.busy = true
		m.stream = streamState{}
		// Keep the event pump alive: only one cmd may read the channel at a time.
		return m, tea.Batch(m.waitEvent(), m.scheduleTick())

	case *rpc.AgentSettledEvent:
		m.busy = false
		m.finishStream()
		m.transcript.MarkDirty()
		return m, tea.Batch(m.waitEvent(), m.refreshStats())

	case *rpc.MessageStartEvent:
		// Assistant content arrives through updates; user messages are added
		// locally when submitted, so nothing to do here.
		return m, m.waitEvent()

	case *rpc.MessageUpdateEvent:
		m.applyStreamUpdate(ev)
		return m, m.waitEvent()

	case *rpc.MessageEndEvent:
		m.applyMessageEnd(ev.Message)
		return m, m.waitEvent()

	case *rpc.ToolExecutionStartEvent:
		m.startTool(ev)
		return m, m.waitEvent()

	case *rpc.ToolExecutionUpdateEvent:
		m.updateTool(ev)
		return m, m.waitEvent()

	case *rpc.ToolExecutionEndEvent:
		m.endTool(ev)
		return m, m.waitEvent()

	case *rpc.QueueUpdateEvent:
		m.queue = *ev
		m.layout()
		return m, m.waitEvent()

	case *rpc.ThinkingLevelChangedEvent:
		m.state.ThinkingLevel = ev.Level
		return m, m.waitEvent()

	case *rpc.SessionInfoChangedEvent:
		m.state.SessionName = ev.Name
		return m, m.waitEvent()

	case *rpc.CompactionStartEvent:
		m.addNotice(fmt.Sprintf("compacting context (%s)…", ev.Reason))
		return m, m.waitEvent()

	case *rpc.CompactionEndEvent:
		switch {
		case ev.ErrorMessage != "":
			m.addError("compaction failed: " + ev.ErrorMessage)
		case ev.Aborted:
			m.addNotice("compaction aborted")
		case ev.Result != nil:
			m.addNotice(fmt.Sprintf("context compacted: %s → ~%s tokens",
				formatTokens(ev.Result.TokensBefore), formatTokens(ev.Result.EstimatedTokensAfter)))
		}
		return m, m.waitEvent()

	case *rpc.AutoRetryStartEvent:
		m.addNotice(fmt.Sprintf("retry %d/%d in %dms: %s",
			ev.Attempt, ev.MaxAttempts, ev.DelayMs, ev.ErrorMessage))
		return m, m.waitEvent()

	case *rpc.AutoRetryEndEvent:
		if !ev.Success && ev.FinalError != "" {
			m.addError("retries exhausted: " + ev.FinalError)
		}
		return m, m.waitEvent()

	case *rpc.SummarizationRetryScheduledEvent:
		m.addNotice(fmt.Sprintf("summarization retry %d/%d in %dms", ev.Attempt, ev.MaxAttempts, ev.DelayMs))
		return m, m.waitEvent()

	case *rpc.BashExecutionUpdateEvent:
		return m, m.waitEvent()

	case *rpc.ExtensionErrorEvent:
		m.addError(fmt.Sprintf("extension error (%s): %s", ev.Event, ev.Error))
		return m, m.waitEvent()

	case *rpc.ExtensionUIRequestEvent:
		return m.handleExtensionUI(ev)

	case *rpc.Response:
		if !ev.Success && ev.Error != "" {
			m.addError(ev.Error)
		}
		return m, m.waitEvent()

	default:
		return m, m.waitEvent()
	}
}

// applyStreamUpdate folds a delta into the live assistant block.
func (m *Model) applyStreamUpdate(update *rpc.MessageUpdateEvent) {
	kind := update.Assistant.Type
	m.stream.acc.Apply(update.Assistant)

	switch {
	case strings.HasPrefix(kind, "thinking_"):
		if m.stream.thinking == nil {
			m.stream.thinking = newBlock(blockThinking, "")
			m.stream.thinking.streaming = true
			m.stream.thinking.collapsed = !m.showThinking
			m.transcript.Add(m.stream.thinking)
		}
		m.stream.thinking.setText(m.stream.acc.Thinking())
		m.stream.dirty = true

	case strings.HasPrefix(kind, "text_"):
		if m.stream.assistant == nil {
			m.stream.assistant = newBlock(blockAssistant, "")
			m.stream.assistant.streaming = true
			m.transcript.Add(m.stream.assistant)
		}
		m.stream.assistant.setText(m.stream.acc.Text())
		m.stream.dirty = true

	case strings.HasPrefix(kind, "toolcall_"):
		// Tool blocks are created by tool_execution_start.
	}

	if update.Usage != nil && update.Usage.TotalTokens > 0 {
		m.stats.Tokens.Total = update.Usage.TotalTokens
	}
}

// applyMessageEnd replaces streamed content with the authoritative message.
func (m *Model) applyMessageEnd(message rpc.Message) {
	switch message.Role {
	case "assistant":
		text := message.Text()
		if text != "" {
			if m.stream.assistant == nil {
				m.stream.assistant = newBlock(blockAssistant, text)
				m.transcript.Add(m.stream.assistant)
			}
			m.stream.assistant.setText(text)
			m.stream.assistant.streaming = false
		} else if m.stream.assistant != nil && strings.TrimSpace(m.stream.assistant.text) == "" {
			m.removeBlock(m.stream.assistant)
			m.stream.assistant = nil
		}
		if m.stream.assistant != nil {
			m.stream.assistant.streaming = false
		}
		if m.stream.thinking != nil {
			if thinking := m.stream.acc.Thinking(); thinking != "" {
				m.stream.thinking.setText(thinking)
			}
			m.stream.thinking.streaming = false
		}
		if thinking := messageThinking(message); thinking != "" && m.stream.thinking == nil {
			block := newBlock(blockThinking, thinking)
			block.collapsed = !m.showThinking
			m.transcript.Add(block)
		}
		m.transcript.MarkDirty()

	case "toolResult":
		m.applyToolResult(message)
	}
}

func (m *Model) finishStream() {
	if m.stream.assistant != nil {
		m.stream.assistant.streaming = false
	}
	if m.stream.thinking != nil {
		m.stream.thinking.streaming = false
	}
	m.stream.dirty = false
	m.stream = streamState{}
}

func (m *Model) removeBlock(target *block) {
	blocks := m.transcript.Blocks()
	for i, existing := range blocks {
		if existing == target {
			m.transcript.blocks = append(blocks[:i], blocks[i+1:]...)
			m.transcript.MarkDirty()
			return
		}
	}
}

// startTool renders a tool call. File mutations get a diff block instead of a
// raw argument dump.
func (m *Model) startTool(event *rpc.ToolExecutionStartEvent) {
	args := rpc.DecodeArgs(event.Args)

	if path, pairs, ok := toolDiff(event.ToolName, args); ok {
		body, truncated := diffBody(pairs, 200)
		added, removed := diffStats(pairs)
		block := newBlock(blockDiff, "")
		block.callID = event.ToolCallID
		block.title = strings.TrimSpace(event.ToolName + " " + path)
		block.meta = fmt.Sprintf("+%d -%d", added, removed)
		block.body = body
		block.collapsed = !m.expanded
		block.streaming = true
		if truncated {
			block.meta += " (truncated)"
		}
		m.transcript.Add(block)
		m.followOutput()
		return
	}

	block := newBlock(blockTool, "")
	block.callID = event.ToolCallID
	block.title = event.ToolName
	block.meta = toolSummary(event.ToolName, args)
	block.status = statusRunning
	block.collapsed = !m.expanded
	block.streaming = true
	m.transcript.Add(block)
	m.followOutput()
}

func (m *Model) updateTool(event *rpc.ToolExecutionUpdateEvent) {
	block := m.transcript.Tool(event.ToolCallID)
	if block == nil {
		return
	}
	if preview := toolResultText(event.PartialResult); preview != "" {
		block.body = preview
		block.invalidate()
	}
}

func (m *Model) endTool(event *rpc.ToolExecutionEndEvent) {
	block := m.transcript.Tool(event.ToolCallID)
	if block == nil {
		return
	}

	if event.IsError {
		block.status = statusError
	}
	if block.kind != blockDiff {
		if event.IsError {
			block.status = statusError
		} else {
			block.status = statusOK
		}
		if body := toolResultText(event.Result); body != "" {
			block.body = body
		}
	}
	if event.DurationMs != nil {
		block.meta = strings.TrimSpace(block.meta + " " + formatDuration(time.Duration(*event.DurationMs*float64(time.Millisecond))))
	}
	if event.IsError && block.kind == blockDiff {
		block.status = statusError
		if body := toolResultText(event.Result); body != "" {
			block.body += "\n! " + body
		}
	}
	block.streaming = false
	block.invalidate()
	m.transcript.MarkDirty()
}

func (m *Model) applyToolResult(message rpc.Message) {
	block := m.transcript.Tool(message.ToolCallID)
	if block == nil {
		return
	}
	if body := toolResultText(mustJSON(message)); body != "" {
		block.body = body
	}
	if message.IsError {
		block.status = statusError
	} else if block.status == statusRunning {
		block.status = statusOK
	}
	block.invalidate()
	m.transcript.MarkDirty()
}

// loadHistory replays messages returned by get_messages into the transcript.
func (m *Model) loadHistory(messages []rpc.Message) {
	m.transcript.Clear()
	for _, message := range messages {
		switch message.Role {
		case "user":
			text := strings.TrimSpace(message.Text())
			if text == "" {
				continue
			}
			m.transcript.Add(newBlock(blockUser, text))
		case "assistant":
			if thinking := messageThinking(message); thinking != "" {
				block := newBlock(blockThinking, thinking)
				block.collapsed = !m.showThinking
				m.transcript.Add(block)
			}
			if text := message.Text(); strings.TrimSpace(text) != "" {
				m.transcript.Add(newBlock(blockAssistant, text))
			}
			for _, call := range message.Tools() {
				args := rpc.DecodeArgs(call.Arguments)
				if path, pairs, ok := toolDiff(call.Name, args); ok {
					body, _ := diffBody(pairs, 200)
					added, removed := diffStats(pairs)
					block := newBlock(blockDiff, "")
					block.callID = call.ID
					block.title = strings.TrimSpace(call.Name + " " + path)
					block.meta = fmt.Sprintf("+%d -%d", added, removed)
					block.body = body
					block.collapsed = true
					m.transcript.Add(block)
					continue
				}
				block := newBlock(blockTool, "")
				block.callID = call.ID
				block.title = call.Name
				block.meta = toolSummary(call.Name, args)
				block.status = statusPending
				block.collapsed = true
				m.transcript.Add(block)
			}
		case "toolResult":
			block := m.transcript.Tool(message.ToolCallID)
			if block == nil {
				continue
			}
			if body := toolResultText(mustJSON(message)); body != "" {
				block.body = body
			}
			if message.IsError {
				block.status = statusError
			} else {
				block.status = statusOK
			}
		}
	}
	m.transcript.ScrollToBottom()
}

// handleKey routes one key press.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.overlay != nil {
		return m.handleOverlayKey(msg)
	}

	key := msg.String()

	// Completion keys win over the editor so the popup behaves predictably.
	if m.completion.visible {
		switch key {
		case "up", "ctrl+p":
			m.completion.Move(-1)
			return m, nil
		case "down", "ctrl+n":
			m.completion.Move(1)
			return m, nil
		case "tab":
			if item, ok := m.completion.Selected(); ok {
				m.acceptCompletion(item)
			}
			return m, nil
		case "enter":
			if item, ok := m.completion.Selected(); ok {
				m.composer.SetText(item.Value)
				m.completion.Hide()
				m.layout()
				return m, m.submit()
			}
		}
	}

	switch key {
	case "ctrl+c":
		if m.busy {
			return m, m.abort()
		}
		if time.Since(m.ctrlCAt) < 1500*time.Millisecond {
			return m, tea.Quit
		}
		m.ctrlCAt = time.Now()
		if !m.composer.Empty() {
			m.composer.Reset()
			m.completion.Hide()
			m.layout()
			return m, nil
		}
		m.setToast("press ctrl+c again to quit")
		return m, nil

	case "ctrl+d":
		if m.composer.Empty() && !m.busy {
			return m, tea.Quit
		}

	case "esc":
		if m.busy {
			return m, m.abort()
		}
		if m.completion.visible {
			m.completion.Hide()
			m.layout()
		}
		return m, nil

	case "enter":
		return m, m.submit()

	case "shift+tab":
		return m, m.cycleThinking()

	case "ctrl+k":
		return m, m.openModelPicker()

	case "ctrl+t":
		m.showThinking = !m.showThinking
		m.applyThinkingVisibility()
		m.setToast(fmt.Sprintf("thinking blocks %s", onOff(m.showThinking)))
		return m, nil

	case "ctrl+o":
		m.expanded = !m.expanded
		m.applyExpansion()
		m.setToast(fmt.Sprintf("tool output %s", onOff(m.expanded)))
		return m, nil

	case "ctrl+l":
		m.transcript.Clear()
		return m, nil

	case "pgup", "pgdown", "home", "end", "ctrl+u":
		return m, m.transcript.Update(msg)
	}

	if key == "up" && !strings.Contains(m.composer.Value(), "\n") {
		if m.composer.HistoryPrev() {
			m.completion.Refresh(m.composer.Value())
			m.layout()
			return m, nil
		}
	}
	if key == "down" && !strings.Contains(m.composer.Value(), "\n") {
		if m.composer.HistoryNext() {
			m.completion.Refresh(m.composer.Value())
			m.layout()
			return m, nil
		}
	}

	beforeHeight := m.composer.Height()
	beforeVisible := m.completion.visible
	beforeCount := len(m.completion.filtered)

	cmd := m.composer.Update(msg)
	m.completion.Refresh(m.composer.Value())

	// The popup changes the reserved body height, so the frame must be
	// re-laid-out whenever it appears, disappears, or resizes.
	if m.composer.Height() != beforeHeight ||
		m.completion.visible != beforeVisible ||
		len(m.completion.filtered) != beforeCount {
		m.layout()
	}
	return m, cmd
}

func (m *Model) acceptCompletion(item completionItem) {
	m.composer.SetText(item.Value + " ")
	m.composer.ta.CursorEnd()
	m.completion.Hide()
	m.layout()
}

func (m *Model) applyThinkingVisibility() {
	for _, block := range m.transcript.Blocks() {
		if block.kind == blockThinking {
			block.collapsed = !m.showThinking
			block.invalidate()
		}
	}
	m.transcript.MarkDirty()
}

func (m *Model) applyExpansion() {
	for _, block := range m.transcript.Blocks() {
		if block.kind == blockTool || block.kind == blockDiff {
			block.collapsed = !m.expanded
			block.invalidate()
		}
	}
	m.transcript.MarkDirty()
}

// submit sends the composer contents.
func (m *Model) submit() tea.Cmd {
	text := strings.TrimSpace(m.composer.Value())
	if text == "" || m.disconnected || m.fatal != "" {
		return nil
	}

	m.composer.PushHistory(text)
	m.composer.Reset()
	m.completion.Hide()
	m.layout()

	if strings.HasPrefix(text, "/") {
		if cmd, handled := m.runSlash(text); handled {
			return cmd
		}
	}

	m.transcript.Add(newBlock(blockUser, text))
	m.followOutput()
	return m.sendPrompt(text)
}

func (m *Model) sendPrompt(text string) tea.Cmd {
	client := m.client
	queued := m.busy
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()

		command := rpc.Prompt(text)
		if queued {
			command = rpc.FollowUp(text)
		}
		response, err := client.DoOK(ctx, command)
		return responseMsg{action: "prompt", resp: response, err: err}
	}
}

// abort clears queued messages, restores them to the editor, then aborts.
func (m *Model) abort() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		msg := abortMsg{}
		if response, err := client.DoOK(ctx, rpc.ClearQueue()); err == nil {
			var payload struct {
				Steering []string `json:"steering"`
				FollowUp []string `json:"followUp"`
			}
			_ = response.DecodeData(&payload)
			msg.restored = append(payload.Steering, payload.FollowUp...)
		} else {
			msg.err = err
		}
		if _, err := client.DoOK(ctx, rpc.Abort()); err != nil && msg.err == nil {
			msg.err = err
		}
		return msg
	}
}

// runSlash executes a client-side command. It reports false when the text is a
// pi command (extension, prompt template, skill) that must be forwarded.
//
// Canonical names always resolve locally. An alias yields to pi when pi
// advertised the same name, so personal commands keep working.
func (m *Model) runSlash(text string) (tea.Cmd, bool) {
	fields := strings.Fields(text)
	name := fields[0]
	argument := strings.TrimSpace(strings.TrimPrefix(text, name))

	target, known := builtinIndex[name]
	if !known || (target.alias && m.piOwns(name)) {
		return nil, false
	}

	switch target.name {
	case "/quit":
		return tea.Quit, true
	case "/clear":
		m.transcript.Clear()
		return nil, true
	case "/help":
		title := "Key bindings"
		if m.opts.Version != "" {
			title = "pie-tui " + m.opts.Version
		}
		m.openOverlay(newPickerOverlay("help", title, m.helpItems()))
		return nil, true
	case "/model":
		if argument != "" {
			return m.setModelByLabel(argument), true
		}
		return m.openModelPicker(), true
	case "/think":
		if argument != "" {
			return m.setThinking(argument), true
		}
		return m.cycleThinking(), true
	case "/new":
		return m.newSession(), true
	case "/resume":
		return m.listSessions(), true
	case "/compact":
		return m.compact(argument), true
	case "/export":
		return m.exportSession(), true
	case "/stats":
		m.statsNotice = true
		return m.refreshStats(), true
	case "/name":
		if argument != "" {
			return m.setSessionName(argument), true
		}
		request := &rpc.ExtensionUIRequestEvent{Title: "Session name", Method: "input", Placeholder: "my-feature-work"}
		overlay := newInputOverlay(request)
		overlay.action = "session-name"
		m.openOverlay(overlay)
		return nil, true
	default:
		return nil, false
	}
}

func (m *Model) helpItems() []pickerItem {
	bindings := [][2]string{
		{"enter", "send prompt (queues a follow-up while running)"},
		{"alt+enter / ctrl+j", "insert a newline"},
		{"ctrl+s", "steer the running turn"},
		{"esc", "interrupt the running turn"},
		{"ctrl+c", "interrupt, twice to quit"},
		{"ctrl+k", "pick a model"},
		{"shift+tab", "cycle thinking level"},
		{"ctrl+t", "toggle thinking blocks"},
		{"ctrl+o", "expand tool output and diffs"},
		{"ctrl+l", "clear the scrollback"},
		{"up / down", "prompt history"},
		{"pgup / pgdn / ctrl+u", "scroll the transcript"},
	}
	items := make([]pickerItem, 0, len(bindings))
	for _, binding := range bindings {
		items = append(items, pickerItem{Value: binding[0], Label: binding[0], Detail: binding[1]})
	}
	for _, command := range builtinCommandTable {
		detail := command.Detail
		if len(command.Aliases) > 0 {
			detail += "  (aliases: " + strings.Join(command.Aliases, ", ") + ")"
		}
		items = append(items, pickerItem{Value: command.Name, Label: command.Name, Detail: detail})
	}
	return items
}

// Command constructors.

func (m *Model) refreshStats() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.GetSessionStats())
		if err != nil {
			return statsMsg{}
		}
		stats, _ := rpc.DecodeTyped[rpc.SessionStats](response)
		return statsMsg{stats: stats}
	}
}

func (m *Model) cycleThinking() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.CycleThinkingLevel())
		return responseMsg{action: "thinking", resp: response, err: err}
	}
}

func (m *Model) setThinking(level string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.SetThinkingLevel(level))
		return responseMsg{action: "thinking", resp: response, err: err}
	}
}

func (m *Model) openModelPicker() tea.Cmd {
	if len(m.models) == 0 {
		m.setToast("no models reported by pi")
		return nil
	}
	items := make([]pickerItem, 0, len(m.models))
	for _, model := range m.models {
		detail := model.Name
		if model.ContextWindow > 0 {
			detail += fmt.Sprintf(" · %s ctx", formatTokens(model.ContextWindow))
		}
		items = append(items, pickerItem{Value: model.Label(), Label: model.Label(), Detail: detail})
	}
	m.openOverlay(newPickerOverlay("model", "Switch model", items))
	return nil
}

func (m *Model) setModelByLabel(label string) tea.Cmd {
	for _, model := range m.models {
		if model.Label() == label || model.ID == label {
			client := m.client
			return func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				response, err := client.DoOK(ctx, rpc.SetModel(model.Provider, model.ID))
				return responseMsg{action: "model", resp: response, err: err}
			}
		}
	}
	m.setToast("unknown model: " + label)
	return nil
}

func (m *Model) newSession() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.NewSession())
		return responseMsg{action: "new_session", resp: response, err: err}
	}
}

func (m *Model) listSessions() tea.Cmd {
	dir, cwd := m.opts.SessionDir, m.opts.CWD
	return func() tea.Msg {
		resolved, err := session.Dir(dir)
		if err != nil {
			return sessionListMsg{err: err}
		}
		entries, err := session.ListForCWD(resolved, cwd, 20)
		if err != nil || len(entries) == 0 {
			// Fall back to the global list when this directory has none.
			entries, err = session.List(resolved, 20)
		}
		return sessionListMsg{entries: entries, err: err}
	}
}

func (m *Model) switchSession(path string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := client.DoOK(ctx, rpc.SwitchSession(path)); err != nil {
			return responseMsg{action: "switch_session", err: err}
		}
		response, err := client.DoOK(ctx, rpc.GetMessages())
		if err != nil {
			return historyMsg{err: err}
		}
		payload, _ := rpc.DecodeTyped[rpc.MessagesPayload](response)
		return historyMsg{messages: payload.Messages}
	}
}

func (m *Model) compact(instructions string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.Compact(instructions))
		return responseMsg{action: "compact", resp: response, err: err}
	}
}

func (m *Model) exportSession() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.ExportHTML(""))
		return responseMsg{action: "export", resp: response, err: err}
	}
}

func (m *Model) setSessionName(name string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.SetSessionName(name))
		return responseMsg{action: "name", resp: response, err: err}
	}
}

// applyResponse folds a command result into the UI.
func (m *Model) applyResponse(msg responseMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.addError(msg.err.Error())
		if msg.action == "prompt" {
			m.busy = false
		}
		return m, nil
	}

	response := msg.resp
	switch msg.action {
	case "prompt":
		var payload struct {
			Disposition string `json:"disposition"`
		}
		if response != nil {
			_ = response.DecodeData(&payload)
		}
		switch payload.Disposition {
		case "queued":
			m.setToast("message queued")
		case "handled":
			m.setToast("handled by pi")
		}

	case "model":
		if model, err := rpc.DecodeTyped[rpc.ModelInfo](response); err == nil && model.ID != "" {
			m.state.Model = &model
			m.setToast("model: " + model.Label())
		}

	case "thinking":
		if level, err := rpc.DecodeTyped[struct {
			Level string `json:"level"`
		}](response); err == nil && level.Level != "" {
			m.state.ThinkingLevel = level.Level
			m.setToast("thinking: " + level.Level)
		} else {
			m.setToast("this model does not support thinking levels")
		}

	case "new_session":
		m.transcript.Clear()
		m.addNotice("started a new session")
		return m, tea.Batch(m.refreshState(), m.refreshStats())

	case "switch_session":
		m.transcript.Clear()
		return m, tea.Batch(m.refreshState(), m.refreshStats())

	case "compact":
		m.setToast("context compacted")
		return m, m.refreshStats()

	case "export":
		if payload, err := rpc.DecodeTyped[rpc.ExportPayload](response); err == nil && payload.Path != "" {
			m.setToast("exported to " + payload.Path)
		} else {
			m.setToast("session exported")
		}

	case "name":
		if response != nil {
			m.setToast("session renamed")
		}

	case "extension-select", "extension-input", "extension-editor":
		return m, nil
	}

	return m, nil
}

func (m *Model) refreshState() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		response, err := client.DoOK(ctx, rpc.GetState())
		if err != nil {
			return responseMsg{action: "state", err: err}
		}
		state, _ := rpc.DecodeTyped[rpc.State](response)
		return stateMsg{state: state}
	}
}

// handleExtensionUI reacts to an extension UI subprotocol record.
func (m *Model) handleExtensionUI(request *rpc.ExtensionUIRequestEvent) (tea.Model, tea.Cmd) {
	switch request.Method {
	case "notify":
		m.setToast(request.Message)
		if request.NotifyType == "error" {
			m.addError(request.Message)
		}
	case "setStatus":
		if request.StatusText == "" {
			delete(m.statuses, request.StatusKey)
		} else {
			m.statuses[request.StatusKey] = request.StatusText
		}
	case "setWidget":
		m.widget = request.WidgetLines
	case "setTitle":
		return m, tea.SetWindowTitle(request.Title)
	case "set_editor_text":
		m.composer.SetText(request.Text)
		m.layout()
	case "select", "confirm", "input", "editor":
		if m.overlay != nil {
			m.dialogs = append(m.dialogs, request)
			return m, m.waitEvent()
		}
		m.openExtensionOverlay(request)
	}
	return m, m.waitEvent()
}

func (m *Model) openExtensionOverlay(request *rpc.ExtensionUIRequestEvent) {
	var overlay *overlay
	switch request.Method {
	case "select":
		overlay = newSelectOverlay(request)
	case "confirm":
		overlay = newConfirmOverlay(request)
	case "input":
		overlay = newInputOverlay(request)
	case "editor":
		overlay = newEditorOverlay(request)
	default:
		return
	}
	m.openOverlay(overlay)
}

func (m *Model) openOverlay(o *overlay) {
	m.overlay = o
	o.Resize(m.width)
	m.completion.Hide()
	m.layout()
}

func (m *Model) closeOverlay() {
	m.overlay = nil
	m.layout()
	if len(m.dialogs) > 0 {
		next := m.dialogs[0]
		m.dialogs = m.dialogs[1:]
		m.openExtensionOverlay(next)
	}
}

// handleOverlayKey routes keys while a modal is open.
func (m *Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	o := m.overlay

	switch key {
	case "esc":
		if o.requestID != "" {
			id := o.requestID
			client := m.client
			m.closeOverlay()
			return m, func() tea.Msg {
				_ = client.Send(rpc.ExtensionUICancelled(id))
				return nil
			}
		}
		m.closeOverlay()
		return m, nil

	case "up", "ctrl+p", "shift+tab":
		if o.kind == overlayPicker || o.kind == overlaySelect {
			o.move(-1)
			return m, nil
		}
	case "down", "ctrl+n", "tab":
		if o.kind == overlayPicker || o.kind == overlaySelect {
			o.move(1)
			return m, nil
		}

	case "enter":
		switch o.kind {
		case overlayPicker, overlaySelect:
			return m, m.selectOverlayItem()
		case overlayConfirm:
			return m, m.answerConfirm(true)
		case overlayInput:
			return m, m.answerInput(o.filter.Value())
		}

	case "y":
		if o.kind == overlayConfirm {
			return m, m.answerConfirm(true)
		}
	case "n":
		if o.kind == overlayConfirm {
			return m, m.answerConfirm(false)
		}

	case "ctrl+s":
		if o.kind == overlayEditor {
			return m, m.answerInput(o.editor.Value())
		}

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, o.Update(msg)
}

func (m *Model) selectOverlayItem() tea.Cmd {
	o := m.overlay
	item, ok := o.selected()
	if !ok {
		return nil
	}

	switch o.kind {
	case overlaySelect:
		id := o.requestID
		client := m.client
		value := item.Value
		m.closeOverlay()
		return func() tea.Msg {
			_ = client.Send(rpc.ExtensionUIValue(id, value))
			return nil
		}

	case overlayPicker:
		action := o.action
		switch action {
		case "model":
			m.closeOverlay()
			return m.setModelByLabel(item.Value)
		case "session":
			m.closeOverlay()
			m.transcript.Clear()
			return m.switchSession(item.Value)
		case "help":
			m.closeOverlay()
			return nil
		case "session-name":
			m.closeOverlay()
			return m.setSessionName(item.Value)
		default:
			m.closeOverlay()
			return nil
		}
	}
	return nil
}

func (m *Model) answerConfirm(confirmed bool) tea.Cmd {
	id := m.overlay.requestID
	client := m.client
	m.closeOverlay()
	return func() tea.Msg {
		_ = client.Send(rpc.ExtensionUIConfirmed(id, confirmed))
		return nil
	}
}

func (m *Model) answerInput(value string) tea.Cmd {
	o := m.overlay
	id := o.requestID
	action := o.action
	client := m.client
	m.closeOverlay()

	if action == "session-name" {
		return m.setSessionName(value)
	}
	return func() tea.Msg {
		_ = client.Send(rpc.ExtensionUIValue(id, value))
		return nil
	}
}

// addNotice appends an informational block.
func (m *Model) addNotice(text string) {
	m.transcript.Add(newBlock(blockNotice, text))
	m.followOutput()
}

// followOutput scrolls to the newest line only when the user is already at the
// bottom, so tool output never yanks the transcript away mid-scroll.
func (m *Model) followOutput() {
	if m.transcript.stickBottom {
		m.transcript.ScrollToBottom()
	}
}

// addError appends an error block and keeps it visible.
func (m *Model) addError(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	m.transcript.Add(newBlock(blockError, text))
	m.transcript.ScrollToBottom()
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}

func messageThinking(message rpc.Message) string {
	var out strings.Builder
	for _, block := range message.Content {
		if block.Type == "thinking" && block.Thinking != "" {
			out.WriteString(block.Thinking)
		}
	}
	return out.String()
}

// toolResultText extracts readable text from a tool result payload.
func toolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if payload.Text != "" {
		return payload.Text
	}
	var parts []string
	for _, item := range payload.Content {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func onOff(value bool) string {
	if value {
		return "shown"
	}
	return "hidden"
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
