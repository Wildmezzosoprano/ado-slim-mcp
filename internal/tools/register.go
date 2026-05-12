package tools

import (
	"ado-slim/internal/ado"

	"github.com/mark3labs/mcp-go/server"
)

// ConfigureAll registers every tool the server exposes. Order matches
// src/index.ts.
func ConfigureAll(s *server.MCPServer, c *ado.Client) {
	configureCoreTools(s, c)
	configureWorkItemTools(s, c)
	configureRepositoryTools(s, c)
	configurePipelineTools(s, c)
	configureSearchTools(s, c)
}
