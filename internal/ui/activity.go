package ui

import (
	"fmt"
	"strings"
	"time"
)

// activityPhase is the broad state of the current run. It is what the activity
// bar names while no tool is executing.
type activityPhase int

const (
	activityIdle activityPhase = iota
	activityWaiting
	activityThinking
	activityStreaming
	activityPreparingTool
	activityRunningTools
)

// activityTool is one tool call currently executing. The bar keeps its argument
// preview so a run that names a file or command stays informative.
type activityTool struct {
	callID string
	name   string
	detail string
}

// activityState is the live run status shown in the single row above the
// composer. It lives outside the transcript, so a long run remains visible
// while the user scrolls history.
type activityState struct {
	phase      activityPhase
	startedAt  time.Time
	tools      []activityTool
	draftTool  string
	compacting bool
	retry      string
}

// begin starts a new run's clock and clears the state of the previous one.
func (a *activityState) begin() {
	*a = activityState{
		phase:     activityWaiting,
		startedAt: time.Now(),
	}
}

// reset folds the bar away when no run is active.
func (a *activityState) reset() { *a = activityState{} }

func (a *activityState) ensureStarted() {
	if a.startedAt.IsZero() {
		a.startedAt = time.Now()
	}
}

func (a *activityState) addTool(callID, name, detail string) {
	a.tools = append(a.tools, activityTool{callID: callID, name: name, detail: detail})
	a.phase = activityRunningTools
	a.draftTool = ""
	a.ensureStarted()
}

// removeTool reports whether a call was running, so ending an unknown call
// cannot move the phase on its own.
func (a *activityState) removeTool(callID string) bool {
	for i, tool := range a.tools {
		if tool.callID != callID {
			continue
		}
		a.tools = append(a.tools[:i], a.tools[i+1:]...)
		if len(a.tools) == 0 && a.phase == activityRunningTools {
			a.phase = activityWaiting
		}
		return true
	}
	return false
}

// activityActive reports whether the animation needs a tick. It follows the
// row rather than busy alone: a manual compaction and a retry run without a
// prompt, and both still need the marker to blink.
func (m *Model) activityActive() bool { return m.activityRows() > 0 }

// activityRows is the space the bar reserves in the frame. The row folds away
// when no run is active, mirroring the collapsible status slot of the desktop
// client, so an idle composer keeps the space it had before.
func (m *Model) activityRows() int {
	if m.activityLabel() == "" {
		return 0
	}
	return 1
}

// activityLabel is the single sentence the bar shows for the current run.
func (m *Model) activityLabel() string {
	a := &m.activity
	switch {
	case a.compacting:
		return "Compacting context…"
	case a.retry != "":
		return a.retry
	case len(a.tools) > 0:
		return activityToolsLabel(a.tools)
	case a.draftTool != "":
		return "Preparing " + a.draftTool + "…"
	case a.phase == activityThinking:
		return "Thinking…"
	case a.phase == activityStreaming:
		return "Writing response…"
	case a.phase == activityWaiting && m.busy:
		return "Waiting for model…"
	case m.busy:
		return "Working…"
	}
	return ""
}

// renderActivity renders the single row directly above the composer. It lives
// outside the transcript, so a long run stays visible while history scrolls.
func (m *Model) renderActivity() string {
	label := m.activityLabel()
	if label == "" || m.width <= 0 {
		return ""
	}

	left := m.activityMarker() + " " + m.styles.activityLabel.Render(label)

	var right string
	if !m.activity.startedAt.IsZero() {
		if elapsed := time.Since(m.activity.startedAt); elapsed >= time.Second {
			right = m.styles.activityMeta.Render(formatElapsed(elapsed))
		}
	}
	if right == "" {
		return truncateLine(left, m.width)
	}

	left = truncateLine(left, maxInt(1, m.width-lipglossWidth(right)-1))
	gap := m.width - lipglossWidth(left) - lipglossWidth(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// activityMarker is the pulsing blue dot in front of the label. It changes both
// colour and glyph, so the blink survives a terminal without colour.
func (m *Model) activityMarker() string {
	const frames = 8 // 8 × 40 ms: a calm ~320 ms half-cycle
	if (m.frame/frames)%2 == 0 {
		return m.styles.activityMarker.Render("●")
	}
	return m.styles.activityMarkerDim.Render("○")
}

// formatElapsed renders the run clock without sub-second noise.
func formatElapsed(elapsed time.Duration) string {
	seconds := int(elapsed / time.Second)
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	seconds -= minutes * 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes -= hours * 60
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}

// activityToolKind groups concurrent calls so the bar stays one line.
type activityToolKind int

const (
	activityToolOther activityToolKind = iota
	activityToolRead
	activityToolSearch
	activityToolList
	activityToolShell
	activityToolEdit
	activityToolWrite
)

var activityShellTools = map[string]bool{
	"bash": true, "shell": true, "sh": true, "zsh": true,
	"powershell": true, "pwsh": true, "cmd": true, "exec": true,
	"exec_command": true, "run_command": true, "run_shell_command": true,
	"execute_command": true, "terminal": true, "local_shell": true,
}

// activityToolKindOf classifies a tool name permissively enough to cover
// extension-decorated names without turning unrelated MCP tools into reads.
func activityToolKindOf(name string) activityToolKind {
	lower := strings.ToLower(strings.TrimSpace(name))
	head := lower
	if i := strings.IndexAny(head, " ("); i >= 0 {
		head = head[:i]
	}
	last := head
	if i := strings.LastIndexAny(last, "._"); i >= 0 {
		last = last[i+1:]
	}

	switch {
	case head == "read" || head == "read_file" || head == "file_read" ||
		last == "read" || strings.HasSuffix(head, "_read_file") || strings.HasSuffix(head, ".read_file"):
		return activityToolRead
	case last == "grep" || last == "find" || last == "search" || last == "glob":
		return activityToolSearch
	case last == "ls" || last == "list" || last == "list_dir" || last == "listdir":
		return activityToolList
	case activityShellTools[head] || activityShellTools[last]:
		return activityToolShell
	case last == "edit" || last == "edit_file" || last == "patch" || last == "apply_patch" ||
		head == "multi_edit" || strings.Contains(head, "str_replace"):
		return activityToolEdit
	case last == "write" || last == "write_file" || last == "create" || head == "write":
		return activityToolWrite
	}
	return activityToolOther
}

// activityToolsLabel names concurrent calls, grouping repeats by kind so the
// row never grows past one line.
func activityToolsLabel(tools []activityTool) string {
	if len(tools) == 1 {
		return activityToolLabel(tools[0])
	}

	type group struct {
		kind  activityToolKind
		tools []activityTool
	}
	var groups []group
	for _, tool := range tools {
		kind := activityToolKindOf(tool.name)
		merged := false
		for i := range groups {
			if groups[i].kind == kind {
				groups[i].tools = append(groups[i].tools, tool)
				merged = true
				break
			}
		}
		if !merged {
			groups = append(groups, group{kind: kind, tools: []activityTool{tool}})
		}
	}

	parts := make([]string, 0, len(groups))
	for _, item := range groups {
		if len(item.tools) == 1 {
			parts = append(parts, activityToolLabel(item.tools[0]))
			continue
		}
		parts = append(parts, activityToolGroupLabel(item.kind, item.tools))
	}
	return strings.Join(parts, " · ")
}

func activityToolGroupLabel(kind activityToolKind, tools []activityTool) string {
	count := len(tools)
	switch kind {
	case activityToolRead:
		return fmt.Sprintf("Reading %d files", count)
	case activityToolSearch:
		return fmt.Sprintf("Searching for %d patterns", count)
	case activityToolList:
		return fmt.Sprintf("Listing %d directories", count)
	case activityToolShell:
		return fmt.Sprintf("Running %d commands", count)
	case activityToolEdit:
		return fmt.Sprintf("Editing %d files", count)
	case activityToolWrite:
		return fmt.Sprintf("Writing %d files", count)
	}

	names := make([]string, 0, count)
	for _, tool := range tools {
		names = append(names, tool.name)
	}
	if count <= 3 {
		return "Running " + strings.Join(names, ", ")
	}
	return fmt.Sprintf("Running %d tools", count)
}

func activityToolLabel(tool activityTool) string {
	detail := strings.TrimSpace(tool.detail)
	switch activityToolKindOf(tool.name) {
	case activityToolRead:
		if detail != "" {
			return "Reading " + detail
		}
		return "Reading…"
	case activityToolSearch:
		if detail != "" {
			return "Searching for " + detail
		}
		return "Searching…"
	case activityToolList:
		if detail != "" {
			return "Listing " + detail
		}
		return "Listing…"
	case activityToolShell:
		if detail != "" {
			return "Running " + detail
		}
		return "Running command…"
	case activityToolEdit:
		if detail != "" {
			return "Editing " + detail
		}
		return "Editing…"
	case activityToolWrite:
		if detail != "" {
			return "Writing " + detail
		}
		return "Writing…"
	}
	if detail != "" {
		return "Running " + tool.name + " " + detail
	}
	return "Running " + tool.name + "…"
}
