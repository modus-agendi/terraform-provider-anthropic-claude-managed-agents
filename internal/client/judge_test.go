package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJudgeVerdict_HappyPathPass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["model"] != "claude-sonnet-4-6" {
			t.Errorf("model = %v, want default claude-sonnet-4-6", body["model"])
		}
		if body["max_tokens"].(float64) != 512 {
			t.Errorf("max_tokens = %v, want default 512", body["max_tokens"])
		}
		if body["system"] != "be a strict evaluator" {
			t.Errorf("system = %v", body["system"])
		}
		messages, ok := body["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("messages = %v", body["messages"])
		}
		msg := messages[0].(map[string]any)
		if msg["role"] != "user" {
			t.Errorf("role = %v", msg["role"])
		}
		content := msg["content"].([]any)
		block := content[0].(map[string]any)
		if block["type"] != "text" || block["text"] != "Did the agent answer 55?" {
			t.Errorf("content = %v", block)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content":[{"type":"text","text":"{\"verdict\":\"PASS\",\"reason\":\"agent answered 55\"}"}]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.JudgeVerdict(context.Background(), JudgeRequest{
		SystemPrompt: "be a strict evaluator",
		UserPrompt:   "Did the agent answer 55?",
	})
	if err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
	if res.Verdict != "PASS" {
		t.Errorf("Verdict = %q", res.Verdict)
	}
	if res.Reason != "agent answered 55" {
		t.Errorf("Reason = %q", res.Reason)
	}
}

func TestJudgeVerdict_HappyPathFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content":[{"type":"text","text":"{\"verdict\":\"FAIL\",\"reason\":\"wrong answer\"}"}]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
	if res.Verdict != "FAIL" {
		t.Errorf("Verdict = %q", res.Verdict)
	}
	if res.Reason != "wrong answer" {
		t.Errorf("Reason = %q", res.Reason)
	}
}

func TestJudgeVerdict_CustomModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["model"] != "claude-opus-4-7" {
			t.Errorf("model = %v, want claude-opus-4-7", body["model"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"verdict\":\"PASS\",\"reason\":\"ok\"}"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.JudgeVerdict(context.Background(), JudgeRequest{
		Model:      "claude-opus-4-7",
		UserPrompt: "x",
	}); err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
}

func TestJudgeVerdict_CustomMaxTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["max_tokens"].(float64) != 1024 {
			t.Errorf("max_tokens = %v", body["max_tokens"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"verdict\":\"PASS\",\"reason\":\"ok\"}"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.JudgeVerdict(context.Background(), JudgeRequest{
		UserPrompt: "x",
		MaxTokens:  1024,
	}); err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
}

func TestJudgeVerdict_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"this is not JSON"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrJudgeMalformed) {
		t.Errorf("expected ErrJudgeMalformed, got %v", err)
	}
	if !strings.Contains(err.Error(), "this is not JSON") {
		t.Errorf("error should include response text, got %v", err)
	}
}

func TestJudgeVerdict_InvalidVerdict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"verdict\":\"MAYBE\",\"reason\":\"unsure\"}"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrJudgeMalformed) {
		t.Errorf("expected ErrJudgeMalformed, got %v", err)
	}
	if !strings.Contains(err.Error(), "MAYBE") {
		t.Errorf("error should mention the bad verdict, got %v", err)
	}
}

func TestJudgeVerdict_APIError(t *testing.T) {
	// Use a 400 (non-retryable) so the typed APIError surfaces directly
	// without being wrapped by retryablehttp's "giving up" envelope.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("request-id", "req_judge")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"oops"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.RequestID != "req_judge" {
		t.Errorf("RequestID = %q", apiErr.RequestID)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
}

func TestJudgeVerdict_ValidatesUserPrompt(t *testing.T) {
	c, err := New(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.JudgeVerdict(context.Background(), JudgeRequest{}); err == nil {
		t.Error("expected error for empty user prompt")
	}
}

func TestJudgeVerdict_ConcatenatesMultipleTextBlocks(t *testing.T) {
	// Defensive: if the judge model emits multiple text blocks we still
	// reassemble the JSON before parsing it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[
			{"type":"text","text":"{\"verdict\":\"PA"},
			{"type":"text","text":"SS\",\"reason\":\"split\"}"}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
	if res.Verdict != "PASS" {
		t.Errorf("Verdict = %q", res.Verdict)
	}
}

func TestJudgeVerdict_PopulatesUsage(t *testing.T) {
	// The Messages API response carries a usage block; JudgeResult must
	// surface it so L5's cost reporter can print real numbers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content":[{"type":"text","text":"{\"verdict\":\"PASS\",\"reason\":\"ok\"}"}],
			"usage":{"input_tokens":482,"output_tokens":34}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
	if res.Usage == nil {
		t.Fatalf("Usage is nil; expected populated")
	}
	if res.Usage.InputTokens != 482 || res.Usage.OutputTokens != 34 {
		t.Errorf("Usage = %+v; want {482, 34}", *res.Usage)
	}
}

func TestJudgeVerdict_MissingUsageStaysNil(t *testing.T) {
	// If the upstream API ever omits the usage block, JudgeResult.Usage
	// must be nil — never silently zero — so callers can distinguish
	// "no data" from "zero tokens used."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content":[{"type":"text","text":"{\"verdict\":\"PASS\",\"reason\":\"ok\"}"}]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	res, err := c.JudgeVerdict(context.Background(), JudgeRequest{UserPrompt: "x"})
	if err != nil {
		t.Fatalf("JudgeVerdict: %v", err)
	}
	if res.Usage != nil {
		t.Errorf("Usage = %+v; want nil", *res.Usage)
	}
}
