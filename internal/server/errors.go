package server

import "fmt"

// ErrorKind categorises errors by what the caller CAN DO, not by origin.
// Follows the "Stop Forwarding Errors, Start Designing Them" philosophy.
type ErrorKind string

const (
	ErrSessionNotFound       ErrorKind = "session_not_found"
	ErrWorkerUnavailable     ErrorKind = "worker_unavailable"
	ErrIDAOperation          ErrorKind = "ida_operation_failed"
	ErrInvalidInput          ErrorKind = "invalid_input"
	ErrDecompilerUnavailable ErrorKind = "decompiler_unavailable"
	ErrInternal              ErrorKind = "internal"
)

// ErrorStatus explicitly declares retry-ability.
type ErrorStatus string

const (
	StatusPermanent ErrorStatus = "permanent"
	StatusTemporary ErrorStatus = "temporary"
)

// ToolError is the single flat error type for MCP tool responses.
type ToolError struct {
	Kind      ErrorKind      `json:"kind"`
	Status    ErrorStatus    `json:"status"`
	Message   string         `json:"message"`
	Operation string         `json:"operation"`
	Context   map[string]any `json:"context,omitempty"`
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Kind, e.Operation, e.Message)
}

// --- Factory functions ---

func sessionNotFound(operation, sessionID string) *ToolError {
	return &ToolError{
		Kind:      ErrSessionNotFound,
		Status:    StatusPermanent,
		Message:   fmt.Sprintf("session %s not found", sessionID),
		Operation: operation,
		Context:   map[string]any{"session_id": sessionID},
	}
}

func workerUnavailable(operation, sessionID string, err error) *ToolError {
	return &ToolError{
		Kind:      ErrWorkerUnavailable,
		Status:    StatusTemporary,
		Message:   "worker process not available",
		Operation: operation,
		Context: map[string]any{
			"session_id": sessionID,
			"detail":     err.Error(),
		},
	}
}

func idaOperationFailed(operation, sessionID string, err error) *ToolError {
	return &ToolError{
		Kind:      ErrIDAOperation,
		Status:    StatusPermanent,
		Message:   err.Error(),
		Operation: operation,
		Context:   map[string]any{"session_id": sessionID},
	}
}

func invalidInput(operation, message string) *ToolError {
	return &ToolError{
		Kind:      ErrInvalidInput,
		Status:    StatusPermanent,
		Message:   message,
		Operation: operation,
	}
}

func internalError(operation string, err error) *ToolError {
	return &ToolError{
		Kind:      ErrInternal,
		Status:    StatusPermanent,
		Message:   err.Error(),
		Operation: operation,
	}
}
