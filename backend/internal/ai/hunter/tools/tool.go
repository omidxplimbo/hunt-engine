package tools

import "context"

// Tool defines the interface for all hunter agent tools.
// Each tool has a name, description, JSON schema for parameters,
// and an Execute method that runs the tool and returns the result.
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]any
	Execute(ctx context.Context, params map[string]any) (string, error)
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Duration int64  `json:"duration_ms"`
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// GetAll returns all registered tools
func (r *ToolRegistry) GetAll() map[string]Tool {
	return r.tools
}

// GetToolDefinitions returns tool definitions formatted for LLM prompt
func (r *ToolRegistry) GetToolDefinitions() []map[string]any {
	defs := make([]map[string]any, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, map[string]any{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters":  t.Schema(),
		})
	}
	return defs
}
