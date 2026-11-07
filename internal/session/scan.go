// Package session inspects pi's session directory so the client can offer a
// resume picker. It reads session files but never writes them: pi owns that
// storage.
package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lihaozhe/pie-tui/internal/config"
)

// Entry is one resumable session file.
type Entry struct {
	// Path is the absolute path of the .jsonl session file.
	Path string
	// ID is the session UUID from the header.
	ID string
	// CWD is the working directory the session was created in.
	CWD string
	// Name is the display name from the newest session_info entry, if any.
	Name string
	// Modified is the file modification time.
	Modified time.Time
	// Size is the file size in bytes.
	Size int64
}

// Label renders a picker title: the session name, else a short ID.
func (e Entry) Label() string {
	if e.Name != "" {
		return e.Name
	}
	if len(e.ID) >= 8 {
		return "session " + e.ID[:8]
	}
	return "session " + e.ID
}

const (
	// DefaultDirEnv overrides the session directory.
	DefaultDirEnv = "PI_CODING_AGENT_SESSION_DIR"
	// headerLimit caps the header JSON we read from a session file.
	headerLimit = 1 << 20
	// tailProbe bounds how much of the file we scan for the display name.
	tailProbe = 64 << 10
)

// Dir resolves the session directory. Precedence matches pi: explicit
// override, then PI_CODING_AGENT_SESSION_DIR, then settings.json sessionDir
// (relative paths resolve from the working directory), then the agent
// directory's sessions folder.
func Dir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv(DefaultDirEnv); env != "" {
		return env, nil
	}
	paths, err := config.ResolvePaths()
	if err != nil {
		return "", err
	}
	if settings, loadErr := config.LoadSettings(paths); loadErr == nil {
		if dir, ok, typeErr := settings.SessionDir(); typeErr == nil && ok {
			if filepath.IsAbs(dir) {
				return dir, nil
			}
			wd, wdErr := os.Getwd()
			if wdErr != nil {
				return "", fmt.Errorf("resolve sessionDir %q: %w", dir, wdErr)
			}
			return filepath.Join(wd, dir), nil
		}
	}
	return paths.Sessions(), nil
}

// List returns the most recently modified sessions, newest first.
//
// Unreadable or malformed session files are skipped: a single corrupt file must
// not break the resume picker.
func List(dir string, limit int) ([]Entry, error) {
	root, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var entries []Entry
	for _, project := range root {
		if !project.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, project.Name()))
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(dir, project.Name(), file.Name())
			entry, err := readEntry(path)
			if err != nil {
				continue
			}
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Modified.After(entries[j].Modified)
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// canonicalDir resolves symlinks so /tmp and /private/tmp compare equal, as
// pi canonicalizes session headers and Pie groups by them.
func canonicalDir(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	if absolute, err := filepath.Abs(path); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(path)
}

// ListForCWD keeps only sessions created in cwd.
func ListForCWD(dir, cwd string, limit int) ([]Entry, error) {
	all, err := List(dir, 0)
	if err != nil {
		return nil, err
	}
	if cwd == "" {
		return all, nil
	}
	clean := canonicalDir(cwd)
	matched := all[:0]
	for _, entry := range all {
		if entry.CWD == "" || canonicalDir(entry.CWD) == clean {
			matched = append(matched, entry)
		}
	}
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

type header struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
}

func readEntry(path string) (Entry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Entry{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Entry{}, err
	}
	defer file.Close()

	line, err := bufio.NewReaderSize(io.LimitReader(file, headerLimit), 64*1024).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return Entry{}, err
	}
	var head header
	if err := json.Unmarshal(line, &head); err != nil {
		return Entry{}, fmt.Errorf("parse header: %w", err)
	}
	if head.Type != "session" || head.ID == "" {
		return Entry{}, errors.New("not a session header")
	}

	return Entry{
		Path:     path,
		ID:       head.ID,
		CWD:      head.CWD,
		Name:     readName(file, info.Size()),
		Modified: info.ModTime(),
		Size:     info.Size(),
	}, nil
}

// readName scans the tail of the file for the newest session_info name. Names
// are only ever set or cleared, so scanning backwards is enough.
func readName(file *os.File, size int64) string {
	start := size - tailProbe
	if start < 0 {
		start = 0
	}
	buf := make([]byte, size-start)
	if _, err := file.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return ""
	}

	name := ""
	for _, line := range strings.Split(string(buf), "\n") {
		if !strings.Contains(line, `"session_info"`) {
			continue
		}
		var info struct {
			Type string  `json:"type"`
			Name *string `json:"name"`
		}
		if err := json.Unmarshal([]byte(line), &info); err != nil || info.Type != "session_info" {
			continue
		}
		if info.Name == nil {
			name = ""
			continue
		}
		name = *info.Name
	}
	return name
}
