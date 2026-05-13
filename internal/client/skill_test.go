package client

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// partFilename extracts the filename parameter directly from the
// Content-Disposition header. mime/multipart.Part.FileName() helpfully strips
// any directory components (via filepath.Base) which would collapse
// `scripts/lint.py` to `lint.py`. We need the raw value to assert that
// subdirectory paths round-trip.
func partFilename(p *multipart.Part) string {
	cd := p.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return ""
	}
	return params["filename"]
}

// readMultipart parses an incoming multipart request and returns the form
// fields + a map of filename -> content. Test helper.
func readMultipart(t *testing.T, r *http.Request) (map[string]string, map[string][]byte) {
	t.Helper()
	ct := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("parse content-type %q: %v", ct, err)
	}
	if !strings.HasPrefix(mediaType, "multipart/form-data") {
		t.Fatalf("content-type = %q, want multipart/form-data", mediaType)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatalf("no boundary in content-type %q", ct)
	}
	mr := multipart.NewReader(r.Body, boundary)
	fields := map[string]string{}
	files := map[string][]byte{}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		b, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		if part.FormName() == "files[]" {
			files[partFilename(part)] = b
		} else {
			fields[part.FormName()] = string(b)
		}
		_ = part.Close()
	}
	return fields, files
}

// ---------------------------------------------------------------------------
// 1. CreateSkill happy path — multipart body with files in subdirs.
// ---------------------------------------------------------------------------

func TestCreateSkill_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/skills" {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("anthropic-beta"); got != "skills-2025-10-02" {
			t.Errorf("anthropic-beta = %q", got)
		}
		fields, files := readMultipart(t, r)
		if fields["display_title"] != "report-builder" {
			t.Errorf("display_title = %q", fields["display_title"])
		}
		if !strings.Contains(string(files["SKILL.md"]), "frontmatter") {
			t.Errorf("SKILL.md content = %q", files["SKILL.md"])
		}
		if string(files["scripts/lint.py"]) != "print('lint')" {
			t.Errorf("scripts/lint.py = %q", files["scripts/lint.py"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"skill_01ABC","type":"skill","source":"custom","display_title":"report-builder","latest_version":"1747000000","created_at":"2026-05-13T10:00:00Z"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	skill, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "report-builder",
		Files: []SkillFile{
			{Path: "SKILL.md", Content: []byte("---\nfrontmatter\n---\n")},
			{Path: "scripts/lint.py", Content: []byte("print('lint')")},
		},
	})
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	if skill.ID != "skill_01ABC" {
		t.Errorf("ID = %q", skill.ID)
	}
	if skill.Source != "custom" {
		t.Errorf("Source = %q", skill.Source)
	}
	if skill.LatestVersion != "1747000000" {
		t.Errorf("LatestVersion = %q", skill.LatestVersion)
	}
}

// ---------------------------------------------------------------------------
// 2. Multipart body construction — direct test of buildSkillMultipart.
// ---------------------------------------------------------------------------

func TestBuildSkillMultipart_Shape(t *testing.T) {
	body, ct, err := buildSkillMultipart("my-skill", []SkillFile{
		{Path: "SKILL.md", Content: []byte("hello")},
		{Path: "nested/file.txt", Content: []byte("world")},
	})
	if err != nil {
		t.Fatalf("buildSkillMultipart: %v", err)
	}
	if !strings.HasPrefix(ct, "multipart/form-data; boundary=") {
		t.Errorf("content type = %q", ct)
	}
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("parse: %v / %q", err, mediaType)
	}
	mr := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	sawTitle := false
	saw := map[string]string{}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("part: %v", err)
		}
		b, _ := io.ReadAll(part)
		if part.FormName() == "display_title" {
			sawTitle = true
			if string(b) != "my-skill" {
				t.Errorf("display_title = %q", b)
			}
		} else {
			if part.FormName() != "files[]" {
				t.Errorf("form name = %q, want files[]", part.FormName())
			}
			saw[partFilename(part)] = string(b)
		}
	}
	if !sawTitle {
		t.Errorf("display_title field missing")
	}
	if saw["SKILL.md"] != "hello" || saw["nested/file.txt"] != "world" {
		t.Errorf("file contents = %+v", saw)
	}
}

func TestBuildSkillMultipart_NoDisplayTitle(t *testing.T) {
	body, _, err := buildSkillMultipart("", []SkillFile{
		{Path: "SKILL.md", Content: []byte("x")},
	})
	if err != nil {
		t.Fatalf("buildSkillMultipart: %v", err)
	}
	if strings.Contains(string(body), "display_title") {
		t.Errorf("body should not include display_title field; got %s", body)
	}
}

// Binary content preserved byte-for-byte even at non-trivial size.
func TestBuildSkillMultipart_BinaryContent(t *testing.T) {
	big := make([]byte, 1024*1024) // 1 MB
	for i := range big {
		big[i] = byte(i % 256)
	}
	body, ct, err := buildSkillMultipart("x", []SkillFile{
		{Path: "SKILL.md", Content: []byte("readme")},
		{Path: "blob.bin", Content: big},
	})
	if err != nil {
		t.Fatalf("buildSkillMultipart: %v", err)
	}
	_, params, _ := mime.ParseMediaType(ct)
	mr := multipart.NewReader(strings.NewReader(string(body)), params["boundary"])
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("part: %v", err)
		}
		if partFilename(part) == "blob.bin" {
			b, _ := io.ReadAll(part)
			if len(b) != len(big) {
				t.Fatalf("len = %d, want %d", len(b), len(big))
			}
			for i := range b {
				if b[i] != big[i] {
					t.Fatalf("byte %d differs: %d vs %d", i, b[i], big[i])
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 3. display_title over 64 chars → client-side validation (preferred over
//    API 400, but we also exercise the API surface separately).
// ---------------------------------------------------------------------------

func TestCreateSkill_DisplayTitleTooLong(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
		w.WriteHeader(http.StatusInternalServerError)
	})))
	long := strings.Repeat("x", 65)
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: long,
		Files:        []SkillFile{{Path: "SKILL.md", Content: []byte("x")}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrSkillDisplayTitleTooLong) {
		t.Errorf("err = %v, want ErrSkillDisplayTitleTooLong", err)
	}
}

// API-side 400 is surfaced as APIError when client-side validation passes
// (e.g. if the API later tightens limits below 64).
func TestCreateSkill_API400Surfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"display_title too long"}}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "okay",
		Files:        []SkillFile{{Path: "SKILL.md", Content: []byte("x")}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("err = %v, want APIError 400", err)
	}
	if !strings.Contains(apiErr.Message, "too long") {
		t.Errorf("message = %q", apiErr.Message)
	}
}

// ---------------------------------------------------------------------------
// 4. Empty files[] — caller-side refusal (no network).
// ---------------------------------------------------------------------------

func TestCreateSkill_EmptyFilesRejected(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "x",
		Files:        nil,
	})
	if !errors.Is(err, ErrSkillFilesEmpty) {
		t.Errorf("err = %v, want ErrSkillFilesEmpty", err)
	}
}

// ---------------------------------------------------------------------------
// 5. Missing SKILL.md — caller-side refusal.
// ---------------------------------------------------------------------------

func TestCreateSkill_MissingSkillMd(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "x",
		Files:        []SkillFile{{Path: "notes.md", Content: []byte("no entrypoint")}},
	})
	if !errors.Is(err, ErrSkillMissingEntrypoint) {
		t.Errorf("err = %v, want ErrSkillMissingEntrypoint", err)
	}
}

// SKILL.md present but only in a subdirectory does NOT count.
func TestCreateSkill_SkillMdInSubdir(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "x",
		Files:        []SkillFile{{Path: "nested/SKILL.md", Content: []byte("readme")}},
	})
	if !errors.Is(err, ErrSkillMissingEntrypoint) {
		t.Errorf("err = %v, want ErrSkillMissingEntrypoint", err)
	}
}

// ---------------------------------------------------------------------------
// 6. >30 MB upload — caller-side refusal.
// ---------------------------------------------------------------------------

func TestCreateSkill_OversizeUpload(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	big := make([]byte, maxSkillUploadBytes+1)
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "x",
		Files: []SkillFile{
			{Path: "SKILL.md", Content: []byte("readme")},
			{Path: "big.bin", Content: big},
		},
	})
	if !errors.Is(err, ErrSkillUploadTooLarge) {
		t.Errorf("err = %v, want ErrSkillUploadTooLarge", err)
	}
}

// ---------------------------------------------------------------------------
// 7. Beta header set on every skill request.
// ---------------------------------------------------------------------------

func TestSkill_BetaHeaderOnAllMethods(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("anthropic-beta"))
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills":
			_, _ = w.Write([]byte(`{"id":"skill_x","source":"custom","display_title":"t","latest_version":"v1","created_at":"2026-05-13T10:00:00Z"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/skill_x":
			_, _ = w.Write([]byte(`{"id":"skill_x","source":"custom"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/skills/skill_x":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/skills/skill_x/versions":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/skills/skill_x/versions":
			_, _ = w.Write([]byte(`{"type":"skill_version","skill_id":"skill_x","version":"v2","created_at":"2026-05-13T10:00:00Z"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/skills/skill_x/versions/v1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	ctx := context.Background()
	if _, err := c.CreateSkill(ctx, SkillCreateRequest{DisplayTitle: "t", Files: []SkillFile{{Path: "SKILL.md", Content: []byte("x")}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetSkill(ctx, "skill_x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListSkills(ctx, ListSkillsParams{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListSkillVersions(ctx, "skill_x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSkillVersion(ctx, "skill_x", SkillVersionCreateRequest{Files: []SkillFile{{Path: "SKILL.md", Content: []byte("x")}}}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSkillVersion(ctx, "skill_x", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSkill(ctx, "skill_x"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 7 {
		t.Fatalf("expected 7 calls, got %d", len(seen))
	}
	for i, v := range seen {
		if v != "skills-2025-10-02" {
			t.Errorf("call %d: anthropic-beta = %q", i, v)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. Beta header NOT applied to non-skill calls (agent endpoint preserves
//    the managed-agents beta).
// ---------------------------------------------------------------------------

func TestSkill_BetaHeaderIsolated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("anthropic-beta")
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/skills"):
			if got != "skills-2025-10-02" {
				t.Errorf("skills endpoint anthropic-beta = %q", got)
			}
			_, _ = w.Write([]byte(`{"id":"skill_x"}`))
		case strings.HasPrefix(r.URL.Path, "/v1/agents"):
			if got != "managed-agents-2026-04-01" {
				t.Errorf("agents endpoint anthropic-beta = %q", got)
			}
			_, _ = w.Write([]byte(`{"id":"agent_x","name":"x","model":{"id":"claude-opus-4-7"},"version":1}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.GetSkill(context.Background(), "skill_x"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetAgent(context.Background(), "agent_x"); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// 9. ListSkills with source=custom filter.
// ---------------------------------------------------------------------------

func TestListSkills_SourceFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("source"); got != "custom" {
			t.Errorf("source query = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("limit query = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"skill_a","source":"custom"}],"has_more":false}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	resp, err := c.ListSkills(context.Background(), ListSkillsParams{Source: "custom", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Errorf("len = %d", len(resp.Data))
	}
}

// ---------------------------------------------------------------------------
// 10. ListSkills pagination — second page via after_id.
// ---------------------------------------------------------------------------

func TestListSkills_Pagination(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			if r.URL.Query().Get("after_id") != "" {
				t.Errorf("first page after_id should be empty")
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"skill_a"},{"id":"skill_b"}],"has_more":true,"last_id":"skill_b"}`))
			return
		}
		if got := r.URL.Query().Get("after_id"); got != "skill_b" {
			t.Errorf("second page after_id = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"skill_c"}],"has_more":false,"last_id":"skill_c"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	page1, err := c.ListSkills(context.Background(), ListSkillsParams{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !page1.HasMore || page1.LastID != "skill_b" {
		t.Errorf("page1 = %+v", page1)
	}
	page2, err := c.ListSkills(context.Background(), ListSkillsParams{Limit: 2, AfterID: page1.LastID})
	if err != nil {
		t.Fatal(err)
	}
	if page2.HasMore || len(page2.Data) != 1 {
		t.Errorf("page2 = %+v", page2)
	}
}

// Also confirm before_id is wired up (covers the symmetric branch).
func TestListSkills_BeforeIDQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("before_id"); got != "skill_z" {
			t.Errorf("before_id = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.ListSkills(context.Background(), ListSkillsParams{BeforeID: "skill_z"}); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// 11. GetSkill — happy + 404.
// ---------------------------------------------------------------------------

func TestGetSkill_HappyAnd404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/skills/skill_ok":
			_, _ = w.Write([]byte(`{"id":"skill_ok","source":"custom","display_title":"ok","latest_version":"v1","created_at":"2026-05-13T10:00:00Z"}`))
		case "/v1/skills/skill_missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"missing"}}`))
		default:
			t.Errorf("path = %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	skill, err := c.GetSkill(context.Background(), "skill_ok")
	if err != nil {
		t.Fatal(err)
	}
	if skill.DisplayTitle != "ok" {
		t.Errorf("DisplayTitle = %q", skill.DisplayTitle)
	}

	_, err = c.GetSkill(context.Background(), "skill_missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(err) = false, err = %v", err)
	}
}

func TestGetSkill_EmptyIDRejected(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	if _, err := c.GetSkill(context.Background(), ""); err == nil {
		t.Errorf("expected error for empty id")
	}
}

// ---------------------------------------------------------------------------
// 12. DeleteSkill — happy + 400 (versions remain) typed error surfaced.
// ---------------------------------------------------------------------------

func TestDeleteSkill_HappyAndConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/skills/skill_ok":
			w.WriteHeader(http.StatusNoContent)
		case "/v1/skills/skill_busy":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"skill has 3 versions; delete them first"}}`))
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.DeleteSkill(context.Background(), "skill_ok"); err != nil {
		t.Fatalf("happy delete: %v", err)
	}
	err := c.DeleteSkill(context.Background(), "skill_busy")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("err = %v, want APIError 400", err)
	}
	if !strings.Contains(apiErr.Message, "delete them first") {
		t.Errorf("message = %q, want server-provided text", apiErr.Message)
	}
}

func TestDeleteSkill_EmptyIDRejected(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	if err := c.DeleteSkill(context.Background(), ""); err == nil {
		t.Errorf("expected error for empty id")
	}
}

// ---------------------------------------------------------------------------
// 13. ListSkillVersions — empty list returns empty slice (not nil).
// ---------------------------------------------------------------------------

func TestListSkillVersions_EmptyAndPopulated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/skills/skill_empty/versions":
			_, _ = w.Write([]byte(`{"data":null,"has_more":false}`))
		case "/v1/skills/skill_full/versions":
			_, _ = w.Write([]byte(`{"data":[{"type":"skill_version","skill_id":"skill_full","version":"v2","created_at":"2026-05-13T10:00:00Z"},{"type":"skill_version","skill_id":"skill_full","version":"v1","created_at":"2026-05-12T10:00:00Z"}],"has_more":false}`))
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)

	empty, err := c.ListSkillVersions(context.Background(), "skill_empty")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Data == nil {
		t.Errorf("Data is nil; want empty slice")
	}
	if len(empty.Data) != 0 {
		t.Errorf("Data len = %d", len(empty.Data))
	}

	full, err := c.ListSkillVersions(context.Background(), "skill_full")
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Data) != 2 {
		t.Errorf("len = %d", len(full.Data))
	}
	if full.Data[0].Version != "v2" {
		t.Errorf("first = %q", full.Data[0].Version)
	}
}

func TestListSkillVersions_EmptyIDRejected(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	if _, err := c.ListSkillVersions(context.Background(), ""); err == nil {
		t.Errorf("expected error for empty id")
	}
}

// ---------------------------------------------------------------------------
// 14. CreateSkillVersion happy path — multipart body parsed, no display_title.
// ---------------------------------------------------------------------------

func TestCreateSkillVersion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/skills/skill_x/versions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fields, files := readMultipart(t, r)
		if _, ok := fields["display_title"]; ok {
			t.Errorf("display_title should be absent on version create; got %q", fields["display_title"])
		}
		if string(files["SKILL.md"]) != "new content" {
			t.Errorf("SKILL.md = %q", files["SKILL.md"])
		}
		_, _ = w.Write([]byte(`{"type":"skill_version","skill_id":"skill_x","version":"1747000000","created_at":"2026-05-13T10:00:00Z"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.CreateSkillVersion(context.Background(), "skill_x", SkillVersionCreateRequest{
		Files: []SkillFile{{Path: "SKILL.md", Content: []byte("new content")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "1747000000" {
		t.Errorf("Version = %q", v.Version)
	}
}

func TestCreateSkillVersion_Validation(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	if _, err := c.CreateSkillVersion(context.Background(), "", SkillVersionCreateRequest{Files: []SkillFile{{Path: "SKILL.md", Content: []byte("x")}}}); err == nil {
		t.Errorf("expected error for empty skill_id")
	}
	if _, err := c.CreateSkillVersion(context.Background(), "skill_x", SkillVersionCreateRequest{}); !errors.Is(err, ErrSkillFilesEmpty) {
		t.Errorf("err = %v, want ErrSkillFilesEmpty", err)
	}
	if _, err := c.CreateSkillVersion(context.Background(), "skill_x", SkillVersionCreateRequest{Files: []SkillFile{{Path: "notes.md", Content: []byte("x")}}}); !errors.Is(err, ErrSkillMissingEntrypoint) {
		t.Errorf("err = %v, want ErrSkillMissingEntrypoint", err)
	}
}

// ---------------------------------------------------------------------------
// 15. DeleteSkillVersion — happy + 404.
// ---------------------------------------------------------------------------

func TestDeleteSkillVersion_HappyAnd404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/skills/skill_x/versions/v1":
			w.WriteHeader(http.StatusNoContent)
		case "/v1/skills/skill_x/versions/v_gone":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"gone"}}`))
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.DeleteSkillVersion(context.Background(), "skill_x", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSkillVersion(context.Background(), "skill_x", "v_gone"); !IsNotFound(err) {
		t.Errorf("IsNotFound = false, err = %v", err)
	}
}

func TestDeleteSkillVersion_Validation(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("API should not be hit")
	})))
	if err := c.DeleteSkillVersion(context.Background(), "", "v1"); err == nil {
		t.Error("expected error for empty skill_id")
	}
	if err := c.DeleteSkillVersion(context.Background(), "skill_x", ""); err == nil {
		t.Error("expected error for empty version")
	}
}

// ---------------------------------------------------------------------------
// 16. Cancellation — context cancelled before request finishes.
// ---------------------------------------------------------------------------

func TestSkill_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel
	_, err := c.GetSkill(ctx, "skill_x")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// 17. Retries on 5xx — server returns 500 twice then 200.
// ---------------------------------------------------------------------------

func TestSkill_RetriesOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"id":"skill_x","source":"custom"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if _, err := c.GetSkill(context.Background(), "skill_x"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// 18. Auth failure — 401 surfaced as APIError.
// ---------------------------------------------------------------------------

func TestSkill_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.CreateSkill(context.Background(), SkillCreateRequest{
		DisplayTitle: "x",
		Files:        []SkillFile{{Path: "SKILL.md", Content: []byte("x")}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := &APIError{}
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("err = %v, want APIError 401", err)
	}
}

// ---------------------------------------------------------------------------
// 19. Malformed JSON on GET — decode error surfaced clearly.
// ---------------------------------------------------------------------------

func TestSkill_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not actually json {{{`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.GetSkill(context.Background(), "skill_x")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("err = %v, want 'decode response' wording", err)
	}
}

// ---------------------------------------------------------------------------
// Helper assertion: shape of the multipart helper error path when no files
// are passed. Direct test for coverage symmetry.
// ---------------------------------------------------------------------------

func TestValidateSkillFiles_OversizeMessage(t *testing.T) {
	big := make([]byte, maxSkillUploadBytes+1)
	err := validateSkillFiles([]SkillFile{
		{Path: "SKILL.md", Content: []byte("x")},
		{Path: "big.bin", Content: big},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrSkillUploadTooLarge) {
		t.Errorf("err = %v", err)
	}
	// The wrapped message should include the actual byte count, surfacing
	// to operators "how far over the limit you are". Don't over-constrain
	// the exact number — just verify "got" + a digit appears.
	if !strings.Contains(err.Error(), "got ") {
		t.Errorf("err message does not include byte count: %v", err)
	}
}
