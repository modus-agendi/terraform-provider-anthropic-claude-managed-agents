// Package client implements the HTTP transport for the Claude Managed Agents
// REST API. It is intentionally narrow: the Terraform provider is the only
// consumer.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// APIError is the typed error returned from every non-2xx response.
type APIError struct {
	StatusCode int    `json:"-"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	RequestID  string `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.RequestID != "" {
		return fmt.Sprintf("anthropic api: status=%d type=%s request_id=%s message=%s",
			e.StatusCode, e.Type, e.RequestID, e.Message)
	}
	return fmt.Sprintf("anthropic api: status=%d type=%s message=%s",
		e.StatusCode, e.Type, e.Message)
}

// IsNotFound reports whether err represents a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsConflict reports whether err represents a 409 from the API.
func IsConflict(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusConflict
	}
	return false
}

// errorEnvelope mirrors the Anthropic-wide error response shape:
//
//	{"type": "error", "error": {"type": "...", "message": "..."}}
type errorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseAPIError(statusCode int, requestID string, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode, RequestID: requestID}

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Type != "" {
		apiErr.Type = env.Error.Type
		apiErr.Message = env.Error.Message
		return apiErr
	}

	apiErr.Type = http.StatusText(statusCode)
	apiErr.Message = string(body)
	return apiErr
}
