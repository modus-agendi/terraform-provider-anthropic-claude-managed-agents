package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

// Skill API limits and validation constants. The 30 MB cap and the
// 64-character display_title cap are documented on the Skills beta; both are
// enforced client-side so the caller gets a clean error before the multipart
// upload is sent to the API.
const (
	maxSkillUploadBytes     = 30 * 1024 * 1024
	maxSkillDisplayTitleLen = 64
	requiredSkillEntrypoint = "SKILL.md"
)

// Skill-specific caller-side validation errors. These are returned BEFORE
// any network call. Callers can match them with errors.Is to give friendly
// CLI/Terraform messages without re-parsing the message body.
var (
	// ErrSkillFilesEmpty is returned when a Create or CreateVersion request
	// has no files attached.
	ErrSkillFilesEmpty = errors.New("client: skill files[] must not be empty")
	// ErrSkillMissingEntrypoint is returned when the file list does not
	// contain a `SKILL.md` at the root.
	ErrSkillMissingEntrypoint = errors.New("client: skill files must include SKILL.md at the root")
	// ErrSkillUploadTooLarge is returned when the total file payload would
	// exceed 30 MB.
	ErrSkillUploadTooLarge = errors.New("client: skill upload exceeds 30 MB limit")
	// ErrSkillDisplayTitleTooLong is returned when display_title is over
	// 64 characters.
	ErrSkillDisplayTitleTooLong = errors.New("client: skill display_title exceeds 64 characters")
)

// CreateSkill issues POST /v1/skills with a multipart/form-data payload.
func (c *Client) CreateSkill(ctx context.Context, req SkillCreateRequest) (*Skill, error) {
	if req.DisplayTitle == "" {
		return nil, fmt.Errorf("client.CreateSkill: display_title is required")
	}
	if len(req.DisplayTitle) > maxSkillDisplayTitleLen {
		return nil, fmt.Errorf("client.CreateSkill: %w (got %d chars)", ErrSkillDisplayTitleTooLong, len(req.DisplayTitle))
	}
	if err := validateSkillFiles(req.Files); err != nil {
		return nil, fmt.Errorf("client.CreateSkill: %w", err)
	}

	body, contentType, err := buildSkillMultipart(req.DisplayTitle, req.Files)
	if err != nil {
		return nil, fmt.Errorf("client.CreateSkill: build multipart: %w", err)
	}

	summary := map[string]any{
		"display_title": req.DisplayTitle,
		"file_count":    len(req.Files),
	}
	var out Skill
	if err := c.doMultipart(ctx, http.MethodPost, "/v1/skills", body, contentType, summary, &out, withHeader("anthropic-beta", skillsBeta)); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSkill issues GET /v1/skills/{id}.
func (c *Client) GetSkill(ctx context.Context, id string) (*Skill, error) {
	if id == "" {
		return nil, fmt.Errorf("client.GetSkill: id is required")
	}
	var out Skill
	path := "/v1/skills/" + url.PathEscape(id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out, withHeader("anthropic-beta", skillsBeta)); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSkill issues DELETE /v1/skills/{id}. The API rejects the call with
// a 400 while any versions remain; callers must DeleteSkillVersion every
// version before invoking this method (see the skill resource's Delete for
// the cascade pattern).
func (c *Client) DeleteSkill(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("client.DeleteSkill: id is required")
	}
	path := "/v1/skills/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, nil, nil, withHeader("anthropic-beta", skillsBeta))
}

// ListSkills issues GET /v1/skills.
func (c *Client) ListSkills(ctx context.Context, params ListSkillsParams) (*ListResponse[Skill], error) {
	q := url.Values{}
	if params.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.BeforeID != "" {
		q.Set("before_id", params.BeforeID)
	}
	if params.AfterID != "" {
		q.Set("after_id", params.AfterID)
	}
	if params.Source != "" {
		q.Set("source", params.Source)
	}
	path := "/v1/skills"
	if encoded := q.Encode(); encoded != "" {
		path = path + "?" + encoded
	}
	var out ListResponse[Skill]
	if err := c.do(ctx, http.MethodGet, path, nil, &out, withHeader("anthropic-beta", skillsBeta)); err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = []Skill{}
	}
	return &out, nil
}

// CreateSkillVersion issues POST /v1/skills/{id}/versions. The shape mirrors
// CreateSkill but has no display_title field — the title is fixed at skill
// creation.
func (c *Client) CreateSkillVersion(ctx context.Context, skillID string, req SkillVersionCreateRequest) (*SkillVersion, error) {
	if skillID == "" {
		return nil, fmt.Errorf("client.CreateSkillVersion: skill_id is required")
	}
	if err := validateSkillFiles(req.Files); err != nil {
		return nil, fmt.Errorf("client.CreateSkillVersion: %w", err)
	}

	body, contentType, err := buildSkillMultipart("", req.Files)
	if err != nil {
		return nil, fmt.Errorf("client.CreateSkillVersion: build multipart: %w", err)
	}

	summary := map[string]any{
		"skill_id":   skillID,
		"file_count": len(req.Files),
	}
	var out SkillVersion
	path := "/v1/skills/" + url.PathEscape(skillID) + "/versions"
	if err := c.doMultipart(ctx, http.MethodPost, path, body, contentType, summary, &out, withHeader("anthropic-beta", skillsBeta)); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSkillVersion issues DELETE /v1/skills/{id}/versions/{version}.
func (c *Client) DeleteSkillVersion(ctx context.Context, skillID, version string) error {
	if skillID == "" {
		return fmt.Errorf("client.DeleteSkillVersion: skill_id is required")
	}
	if version == "" {
		return fmt.Errorf("client.DeleteSkillVersion: version is required")
	}
	path := "/v1/skills/" + url.PathEscape(skillID) + "/versions/" + url.PathEscape(version)
	return c.do(ctx, http.MethodDelete, path, nil, nil, withHeader("anthropic-beta", skillsBeta))
}

// ListSkillVersions issues GET /v1/skills/{id}/versions.
func (c *Client) ListSkillVersions(ctx context.Context, skillID string) (*ListResponse[SkillVersion], error) {
	if skillID == "" {
		return nil, fmt.Errorf("client.ListSkillVersions: skill_id is required")
	}
	path := "/v1/skills/" + url.PathEscape(skillID) + "/versions"
	var out ListResponse[SkillVersion]
	if err := c.do(ctx, http.MethodGet, path, nil, &out, withHeader("anthropic-beta", skillsBeta)); err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = []SkillVersion{}
	}
	return &out, nil
}

// validateSkillFiles enforces the three caller-side invariants: non-empty,
// includes SKILL.md at the root, total size ≤ 30 MB.
func validateSkillFiles(files []SkillFile) error {
	if len(files) == 0 {
		return ErrSkillFilesEmpty
	}
	var total int
	var hasEntrypoint bool
	for _, f := range files {
		total += len(f.Content)
		if f.Path == requiredSkillEntrypoint {
			hasEntrypoint = true
		}
	}
	if !hasEntrypoint {
		return ErrSkillMissingEntrypoint
	}
	if total > maxSkillUploadBytes {
		return fmt.Errorf("%w (got %d bytes)", ErrSkillUploadTooLarge, total)
	}
	return nil
}

// buildSkillMultipart encodes the multipart body for POST /v1/skills and
// POST /v1/skills/{id}/versions. If displayTitle is non-empty it is included
// as a `display_title` form field; otherwise the body is files-only (the
// shape used by the version-create endpoint). Every file is emitted as a
// `files[]` part with `filename` set to the POSIX-style relative path.
//
// The returned contentType already includes the multipart boundary and must
// be set verbatim on the request's Content-Type header.
func buildSkillMultipart(displayTitle string, files []SkillFile) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if displayTitle != "" {
		if err := w.WriteField("display_title", displayTitle); err != nil {
			return nil, "", fmt.Errorf("write display_title: %w", err)
		}
	}

	for _, f := range files {
		// Build a part header manually so we can preserve the exact
		// `filename` (with subdirectories) the server expects. The
		// stdlib's CreateFormFile escapes path separators differently.
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="files[]"; filename=%q`, f.Path))
		h.Set("Content-Type", "application/octet-stream")
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, "", fmt.Errorf("create part %q: %w", f.Path, err)
		}
		if _, err := part.Write(f.Content); err != nil {
			return nil, "", fmt.Errorf("write part %q: %w", f.Path, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	contentType := w.FormDataContentType()
	// Sanity: never return an obviously-broken content type.
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		return nil, "", fmt.Errorf("unexpected content type %q", contentType)
	}
	return buf.Bytes(), contentType, nil
}
