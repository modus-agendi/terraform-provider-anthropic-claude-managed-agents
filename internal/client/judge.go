package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// defaultJudgeModel is the model used by JudgeVerdict when the caller
// leaves req.Model empty. The L5 scenarios suite uses Sonnet — plenty
// capable for PASS/FAIL grading of concrete answers and cheaper than
// Opus.
const defaultJudgeModel = "claude-sonnet-4-6"

// defaultJudgeMaxTokens is the cap applied when req.MaxTokens is zero.
// 512 is enough room for a JSON verdict + a sentence of reasoning.
const defaultJudgeMaxTokens = 512

// ErrJudgeMalformed is returned when the judge model returned a 2xx but
// the content block could not be parsed as the JudgeResult schema, or
// the verdict field was not exactly "PASS" or "FAIL". The wrapped error
// includes the raw response text to aid debugging.
var ErrJudgeMalformed = errors.New("client: judge response malformed")

// JudgeVerdict wraps POST /v1/messages with a fixed request shape: a
// single user message under the supplied system prompt, capped at
// MaxTokens. The judge is expected to reply with exactly one JSON object
// matching JudgeResult in its only text content block.
//
// Uses the stable Messages API (no managed-agents beta header). On
// success the parsed JudgeResult is returned; on any error (transport,
// non-2xx response, malformed JSON, or verdict outside the allowed
// set), the error is surfaced verbatim.
func (c *Client) JudgeVerdict(ctx context.Context, req JudgeRequest) (*JudgeResult, error) {
	if req.UserPrompt == "" {
		return nil, fmt.Errorf("client.JudgeVerdict: user_prompt is required")
	}
	model := req.Model
	if model == "" {
		model = defaultJudgeModel
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultJudgeMaxTokens
	}

	type contentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type messageParam struct {
		Role    string         `json:"role"`
		Content []contentBlock `json:"content"`
	}
	body := struct {
		Model     string         `json:"model"`
		MaxTokens int            `json:"max_tokens"`
		System    string         `json:"system,omitempty"`
		Messages  []messageParam `json:"messages"`
	}{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages: []messageParam{{
			Role: "user",
			Content: []contentBlock{{
				Type: "text",
				Text: req.UserPrompt,
			}},
		}},
	}

	var resp struct {
		Content []contentBlock `json:"content"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/messages", body, &resp); err != nil {
		return nil, err
	}

	var raw string
	for _, block := range resp.Content {
		if block.Type == "text" {
			raw += block.Text
		}
	}

	var result JudgeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("%w: unmarshal verdict JSON from %q: %w", ErrJudgeMalformed, raw, err)
	}
	if result.Verdict != "PASS" && result.Verdict != "FAIL" {
		return nil, fmt.Errorf("%w: verdict must be PASS or FAIL, got %q (raw: %s)", ErrJudgeMalformed, result.Verdict, raw)
	}
	return &result, nil
}
