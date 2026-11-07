package config

import "path/filepath"

// MCPFile is a snapshot of one mcp.json (global or project).
type MCPFile struct {
	path string
	root *Value
}

// MCPServer is one entry of mcpServers.
type MCPServer struct {
	Name        string
	Command     string
	Args        []string
	URL         string
	Enabled     bool
	Exposure    string
	Description string
	Raw         *Value
}

// LoadMCP reads the global mcp.json. A missing file reads as empty.
func LoadMCP(paths Paths) (*MCPFile, error) {
	return loadMCPFile(paths.MCP())
}

// LoadProjectMCP reads a project's .pi/mcp.json.
func LoadProjectMCP(projectDir string) (*MCPFile, error) {
	return loadMCPFile(filepath.Join(projectDir, ".pi", "mcp.json"))
}

func loadMCPFile(path string) (*MCPFile, error) {
	doc, err := LoadObject(path)
	if err != nil {
		return nil, err
	}
	return &MCPFile{path: path, root: doc.Root()}, nil
}

// Path returns the file path.
func (m *MCPFile) Path() string { return m.path }

// Root returns the current tree.
func (m *MCPFile) Root() *Value { return m.root }

// Servers returns the configured servers in file order. A malformed entry is
// still listed, with its raw value available, so the panel can show it.
func (m *MCPFile) Servers() []MCPServer {
	container := m.root.Get("mcpServers")
	if container == nil || !container.IsObject() {
		return nil
	}
	out := make([]MCPServer, 0, container.Len())
	for _, name := range container.Keys() {
		entry := container.Get(name)
		server := MCPServer{Name: name, Enabled: true, Raw: entry, Exposure: "auto"}
		if entry == nil || !entry.IsObject() {
			out = append(out, server)
			continue
		}
		server.Command, _ = entry.Get("command").AsString()
		server.Args = entry.Get("args").StringsOr(nil)
		server.URL, _ = entry.Get("url").AsString()
		server.Description, _ = entry.Get("description").AsString()
		server.Exposure, _ = entry.Get("exposure").AsString()
		if server.Exposure == "" {
			server.Exposure = "auto"
		}
		if enabled := entry.Get("enabled"); enabled != nil {
			server.Enabled = enabled.BoolOr(true)
		}
		out = append(out, server)
	}
	return out
}

// Find returns one server by name.
func (m *MCPFile) Find(name string) (MCPServer, bool) {
	for _, server := range m.Servers() {
		if server.Name == name {
			return server, true
		}
	}
	return MCPServer{}, false
}
