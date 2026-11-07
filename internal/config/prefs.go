package config

import "fmt"

// Terminal-only preference values.
const (
	DiffUnified = "unified"
	DiffSplit   = "split"
)

// TUIConfig is pie-tui's own preferences file. Anything both clients share
// lives in pi's files instead; this holds only choices a browser does not have.
type TUIConfig struct {
	path string
	root *Value
}

// LoadTUIConfig reads pie-tui-settings.json. A missing file reads as defaults.
func LoadTUIConfig(paths Paths) (*TUIConfig, error) {
	doc, err := LoadObject(paths.TUIConfig())
	if err != nil {
		return nil, err
	}
	return &TUIConfig{path: paths.TUIConfig(), root: doc.Root()}, nil
}

// Path returns the file path.
func (c *TUIConfig) Path() string { return c.path }

// Theme returns the saved theme id, or "auto".
func (c *TUIConfig) Theme() string {
	return c.root.Get("theme").StringOr("auto")
}

// SetTheme saves the theme id.
func (c *TUIConfig) SetTheme(theme string) error {
	if theme == "" {
		return fmt.Errorf("theme: empty id")
	}
	return c.update(func(root *Value) { root.SetString("theme", theme) })
}

// EnterToSend reports whether Enter submits the composer. Default true, the
// terminal convention; Ctrl+Enter inserts a newline when this is false.
func (c *TUIConfig) EnterToSend() bool {
	value := c.root.Get("enterToSend")
	if value == nil {
		return true
	}
	return value.BoolOr(true)
}

// SetEnterToSend saves the composer's submit behavior.
func (c *TUIConfig) SetEnterToSend(enabled bool) error {
	return c.update(func(root *Value) { root.SetBool("enterToSend", enabled) })
}

// DiffLayout returns DiffUnified or DiffSplit.
func (c *TUIConfig) DiffLayout() string {
	layout := c.root.Get("diffLayout").StringOr(DiffUnified)
	if layout != DiffSplit {
		return DiffUnified
	}
	return layout
}

// SetDiffLayout saves the diff layout.
func (c *TUIConfig) SetDiffLayout(layout string) error {
	if layout != DiffUnified && layout != DiffSplit {
		return fmt.Errorf("diffLayout: unknown layout %q", layout)
	}
	return c.update(func(root *Value) { root.SetString("diffLayout", layout) })
}

// GoalEnabled reports whether the goal panel was enabled. It does not mean the
// goal extension is installed; the caller checks availability separately.
func (c *TUIConfig) GoalEnabled() bool {
	value := c.root.Get("goalEnabled")
	if value == nil {
		return false
	}
	return value.BoolOr(false)
}

// SetGoalEnabled saves the goal panel preference.
func (c *TUIConfig) SetGoalEnabled(enabled bool) error {
	return c.update(func(root *Value) { root.SetBool("goalEnabled", enabled) })
}

// LastCWD returns the directory used the last time, or "".
func (c *TUIConfig) LastCWD() string {
	return c.root.Get("lastCwd").StringOr("")
}

// SetLastCWD remembers the working directory for the next start.
func (c *TUIConfig) SetLastCWD(dir string) error {
	return c.update(func(root *Value) {
		if dir == "" {
			root.Delete("lastCwd")
			return
		}
		root.SetString("lastCwd", dir)
	})
}

// ToolPreset returns the last chosen preset selection, or "configured".
func (c *TUIConfig) ToolPreset() string {
	return c.root.Get("toolPreset").StringOr("configured")
}

// SetToolPreset saves the last chosen preset selection.
func (c *TUIConfig) SetToolPreset(selection string) error {
	if selection == "" {
		return fmt.Errorf("toolPreset: empty selection")
	}
	return c.update(func(root *Value) { root.SetString("toolPreset", selection) })
}

// ShowToolCards reports whether tool calls render as cards rather than compact
// lines. Default true.
func (c *TUIConfig) ShowToolCards() bool {
	value := c.root.Get("showToolCards")
	if value == nil {
		return true
	}
	return value.BoolOr(true)
}

// SetShowToolCards saves the tool rendering preference.
func (c *TUIConfig) SetShowToolCards(enabled bool) error {
	return c.update(func(root *Value) { root.SetBool("showToolCards", enabled) })
}

func (c *TUIConfig) update(fn func(root *Value)) error {
	err := Update(c.path, func(root *Value) error {
		fn(root)
		return nil
	})
	if err != nil {
		return err
	}
	return c.Reload()
}

// Reload refreshes the snapshot.
func (c *TUIConfig) Reload() error {
	doc, err := LoadObject(c.path)
	if err != nil {
		return err
	}
	c.root = doc.Root()
	return nil
}
