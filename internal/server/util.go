package server

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// logAndSanitizeError logs the full error server-side and returns a sanitized error for the MCP client.
// Deprecated: prefer handleToolError for structured error responses.
func (s *Server) logAndSanitizeError(context string, err error) error {
	s.logger.Printf("[Error] %s: %v", context, err)

	return fmt.Errorf("%s failed", context)
}

// handleToolError logs the structured ToolError and returns an MCP CallToolResult with
// the serialised JSON body, so MCP clients can programmatically recover using the kind/status fields.
func (s *Server) handleToolError(terr *ToolError) (*mcp.CallToolResult, any, error) {
	s.logger.Printf("[Error] %s", terr.Error())
	body, _ := s.marshalJSON(terr)
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
	}, nil, nil
}

func (s *Server) logToolInvocation(tool, sessionID string, details map[string]interface{}) {
	if details == nil {
		details = map[string]interface{}{}
	}
	if sessionID != "" {
		details["session"] = sessionID
	}
	s.logger.Printf("[Tool] %s %v", tool, details)
}

// marshalJSON marshals v to JSON, using indentation when debug mode is enabled
func (s *Server) marshalJSON(v interface{}) ([]byte, error) {

	return json.MarshalIndent(v, "", "  ")
}

func (s *Server) debugf(format string, args ...interface{}) {
	if s.debug {
		s.logger.Printf("[DEBUG] "+format, args...)
	}
}
