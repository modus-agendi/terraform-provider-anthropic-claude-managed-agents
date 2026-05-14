package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateSession issues POST /v1/sessions. AgentID is required; the upstream
// API requires EnvironmentID in practice but tolerates other combinations
// (e.g. pinning to a specific agent version) that this client does not
// currently expose.
func (c *Client) CreateSession(ctx context.Context, req SessionCreateRequest) (*Session, error) {
	if req.AgentID == "" {
		return nil, fmt.Errorf("client.CreateSession: agent_id is required")
	}
	var out Session
	if err := c.do(ctx, http.MethodPost, "/v1/sessions", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSession issues GET /v1/sessions/{id}.
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetSession: id is required")
	}
	var out Session
	path := "/v1/sessions/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveSession issues POST /v1/sessions/{id}/archive. The upstream API
// also exposes DELETE /v1/sessions/{id}; archive is the safer terminal
// operation because it preserves event history for post-mortem.
func (c *Client) ArchiveSession(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.ArchiveSession: id is required")
	}
	path := "/v1/sessions/" + url.PathEscape(id) + "/archive"
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// sessionEventsBody is the request envelope for POST /v1/sessions/{id}/events.
type sessionEventsBody struct {
	Events []UserEvent `json:"events"`
}

// PostSessionEvents issues POST /v1/sessions/{id}/events with the given
// user events wrapped in an `{"events": [...]}` envelope. Each event is
// expected to set Type (e.g. "user.message"); for `user.message` events,
// Content carries the text blocks.
func (c *Client) PostSessionEvents(ctx context.Context, sessionID string, events []UserEvent) error {
	if sessionID == "" {
		return fmt.Errorf("client.PostSessionEvents: session_id is required")
	}
	if len(events) == 0 {
		return fmt.Errorf("client.PostSessionEvents: events must not be empty")
	}
	path := "/v1/sessions/" + url.PathEscape(sessionID) + "/events"
	return c.do(ctx, http.MethodPost, path, sessionEventsBody{Events: events}, nil)
}

// ListSessionEventsParams holds query parameters for ListSessionEvents.
//
// After is the cursor for tailing — the API returns only events with an
// id strictly greater than After. Types filters the response to a fixed
// set of event types (e.g. []{"agent.message", "session.status_idle"}).
type ListSessionEventsParams struct {
	After string
	Types []string
}

// ListSessionEvents issues GET /v1/sessions/{id}/events.
//
// The harness uses this as a poll loop: pass the last seen event id as
// `After` to retrieve only newer events. Pagination follows the standard
// ListResponse[T] cursor envelope.
func (c *Client) ListSessionEvents(ctx context.Context, sessionID string, params ListSessionEventsParams) (*ListResponse[SessionEvent], error) {
	if sessionID == "" {
		return nil, fmt.Errorf("client.ListSessionEvents: session_id is required")
	}
	q := url.Values{}
	if params.After != "" {
		q.Set("after", params.After)
	}
	for _, t := range params.Types {
		q.Add("types[]", t)
	}
	path := "/v1/sessions/" + url.PathEscape(sessionID) + "/events"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[SessionEvent]
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = []SessionEvent{}
	}
	return &out, nil
}
