package scenarios

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/andasv/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

// init sets the namespace plugin-testing uses to construct the implicit
// required_providers block for each test factory. Matches the provider's
// real registry namespace so YAML configs can reference
// `claude-managed-agents` as the local provider name and still resolve
// to `andasv/anthropic-claude-managed-agents`.
func init() {
	_ = os.Setenv("TF_ACC_PROVIDER_NAMESPACE", "andasv")
}

// TestScenarios is the live entrypoint. Gated by TF_ACC_SCENARIOS=1 — when
// unset, the test skips so unit-test runs (`make test`) don't try to bill
// inference tokens.
//
// Scenarios are discovered by walking ./scenarios for *.yaml. Each file
// becomes one t.Run subtest. After all subtests complete, the cost
// summary table is printed to stdout.
func TestScenarios(t *testing.T) {
	if os.Getenv("TF_ACC_SCENARIOS") != "1" {
		t.Skip("set TF_ACC_SCENARIOS=1 to run live scenario tests")
	}

	scns, err := LoadDir(filepath.Join("scenarios"))
	if err != nil {
		t.Fatalf("scenarios: load: %v", err)
	}
	if len(scns) == 0 {
		t.Fatalf("scenarios: no YAML files found under ./scenarios")
	}

	var agg aggregator
	for _, scn := range scns {
		t.Run(scn.Name, func(t *testing.T) {
			runScenario(t, scn, &agg)
		})
	}

	var summary strings.Builder
	printSummary(&summary, agg.results)
	// Write to both stdout (CI log) and t.Log (per-test artifact).
	fmt.Println(summary.String())
	t.Log("\n" + summary.String())
}

// --- loader unit tests --------------------------------------------------

func TestLoad_validMinimal(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "minimal.yaml")
	writeFile(t, yamlPath, `
name: minimal
terraform_config: "resource \"a\" \"b\" {}"
question: q
rubric: r
`)
	s, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name != "minimal" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.Timeout != defaultTimeout {
		t.Errorf("Timeout default = %v want %v", s.Timeout, defaultTimeout)
	}
	if s.JudgeModel != defaultJudgeModel {
		t.Errorf("JudgeModel default = %q", s.JudgeModel)
	}
}

func TestLoad_missingRequiredField(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing name", "terraform_config: x\nquestion: q\nrubric: r\n", "name"},
		{"missing terraform_config", "name: n\nquestion: q\nrubric: r\n", "terraform_config"},
		{"missing question", "name: n\nterraform_config: x\nrubric: r\n", "question"},
		{"missing rubric", "name: n\nterraform_config: x\nquestion: q\n", "rubric"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scn.yaml")
			writeFile(t, path, tc.body)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q missing %q", err, tc.want)
			}
		})
	}
}

func TestLoad_unknownCheckKey(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	path := filepath.Join(t.TempDir(), "scn.yaml")
	writeFile(t, path, `
name: n
terraform_config: x
question: q
rubric: r
trajectory_checks:
  - bogus_check: foo
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bogus_check") {
		t.Errorf("error %q does not mention bad key", err)
	}
	// All four registered keys must appear in the error so the user
	// can self-correct.
	for _, want := range []string{
		"require_event", "require_terminal_stop_reason",
		"require_no_session_errors", "require_tool_use_named",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing valid key %q", err, want)
		}
	}
}

func TestLoad_typedArgValidation(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	path := filepath.Join(t.TempDir(), "scn.yaml")
	// require_event needs a string; give it an int.
	writeFile(t, path, `
name: n
terraform_config: x
question: q
rubric: r
trajectory_checks:
  - require_event: 42
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "require_event") {
		t.Errorf("error missing check name: %v", err)
	}
}

// TestLoadDir_shippedScenarios confirms every YAML file shipped under
// scenarios/ loads cleanly. Catches "PR-2 ships an unparseable scenario"
// without burning tokens.
func TestLoadDir_shippedScenarios(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	scns, err := LoadDir("scenarios")
	if err != nil {
		t.Fatalf("LoadDir scenarios/: %v", err)
	}
	if len(scns) == 0 {
		t.Fatal("expected at least one shipped scenario")
	}
	for _, s := range scns {
		if s.Name == "" || s.Question == "" || s.Rubric == "" {
			t.Errorf("scenario %s has empty required field", s.SourcePath())
		}
	}
}

func TestLoadDir_lexicalOrder(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.yaml"), validYAML("bb"))
	writeFile(t, filepath.Join(dir, "a.yaml"), validYAML("aa"))
	writeFile(t, filepath.Join(dir, "ignore.txt"), "should not be loaded")
	scns, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(scns) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(scns))
	}
	if scns[0].Name != "aa" || scns[1].Name != "bb" {
		t.Errorf("order: %s, %s", scns[0].Name, scns[1].Name)
	}
}

// --- check builders -----------------------------------------------------

func TestCheck_requireEvent(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	chk, err := buildRequireEvent("agent.tool_use")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := chk(synthEvents(t, "user.message", "agent.tool_use")); err != nil {
		t.Errorf("present: %v", err)
	}
	if err := chk(synthEvents(t, "user.message", "agent.message")); err == nil {
		t.Error("expected failure on missing event")
	}
	// Wrong arg type
	if _, err := buildRequireEvent(123); err == nil {
		t.Error("expected error for non-string arg")
	}
	if _, err := buildRequireEvent(""); err == nil {
		t.Error("expected error for empty arg")
	}
}

func TestCheck_requireTerminalStopReason(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	chk, err := buildRequireTerminalStopReason("end_turn")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	good := []client.SessionEvent{rawEvent(t, "ev1", "session.status_idle", `{"id":"ev1","type":"session.status_idle","stop_reason":{"type":"end_turn"}}`)}
	if err := chk(good); err != nil {
		t.Errorf("good case: %v", err)
	}
	bad := []client.SessionEvent{rawEvent(t, "ev1", "session.status_idle", `{"id":"ev1","type":"session.status_idle","stop_reason":{"type":"requires_action"}}`)}
	if err := chk(bad); err == nil {
		t.Error("expected failure on wrong stop_reason")
	}
	if _, err := buildRequireTerminalStopReason(0); err == nil {
		t.Error("expected error for non-string arg")
	}
}

func TestCheck_requireNoSessionErrors(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	chk, err := buildRequireNoSessionErrors(true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := chk(synthEvents(t, "user.message", "agent.message")); err != nil {
		t.Errorf("clean trajectory: %v", err)
	}
	bad := []client.SessionEvent{rawEvent(t, "ev1", "session.error", `{"id":"ev1","type":"session.error","error":{"message":"boom"}}`)}
	if err := chk(bad); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error mentioning boom: %v", err)
	}
}

func TestCheck_requireToolUseNamed(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	chk, err := buildRequireToolUseNamed("code_execution")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	good := []client.SessionEvent{rawEvent(t, "ev1", "agent.tool_use", `{"id":"ev1","type":"agent.tool_use","name":"code_execution"}`)}
	if err := chk(good); err != nil {
		t.Errorf("good: %v", err)
	}
	bad := []client.SessionEvent{rawEvent(t, "ev1", "agent.tool_use", `{"id":"ev1","type":"agent.tool_use","name":"web_fetch"}`)}
	if err := chk(bad); err == nil {
		t.Error("expected failure on wrong tool name")
	}
	if _, err := buildRequireToolUseNamed(nil); err == nil {
		t.Error("expected error for nil arg")
	}
}

// --- pricing ------------------------------------------------------------

func TestPricing_knownAndUnknown(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	// 1M regular in + 1M out on opus = $15 + $75 = $90
	got := estimateUSD("claude-opus-4-7", 1_000_000, 0, 0, 1_000_000)
	if got != 90.0 {
		t.Errorf("opus cost = %f want 90", got)
	}
	// Sonnet: $3 + $15 = $18
	got = estimateUSD("claude-sonnet-4-6", 1_000_000, 0, 0, 1_000_000)
	if got != 18.0 {
		t.Errorf("sonnet cost = %f want 18", got)
	}
	// Cache create on opus is 1.25× input: 1M tokens → $15 × 1.25 = $18.75
	got = estimateUSD("claude-opus-4-7", 0, 1_000_000, 0, 0)
	if got != 18.75 {
		t.Errorf("opus cache-create cost = %f want 18.75", got)
	}
	// Cache read on opus is 0.10× input: 1M tokens → $15 × 0.10 = $1.50
	got = estimateUSD("claude-opus-4-7", 0, 0, 1_000_000, 0)
	if got != 1.5 {
		t.Errorf("opus cache-read cost = %f want 1.50", got)
	}
	// Unknown model: 0, no panic
	got = estimateUSD("not-a-model", 1_000_000, 0, 0, 1_000_000)
	if got != 0 {
		t.Errorf("unknown cost = %f want 0", got)
	}
	if isPriced("not-a-model") {
		t.Error("isPriced should be false for unknown model")
	}
	if !isPriced("claude-opus-4-7") {
		t.Error("isPriced should be true for opus")
	}
}

// --- cost summary printer ----------------------------------------------

func TestPrintSummary_format(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	results := []scenarioResult{
		{Name: "alpha", Pass: true, DurationSec: 30, Model: "claude-opus-4-7", SessionInput: 1000, SessionOutput: 500, JudgeModel: "claude-sonnet-4-6", JudgeIn: 50, JudgeOut: 20},
		{Name: "beta", Pass: false, DurationSec: 90, Model: "claude-opus-4-7", SessionInput: 500, SessionCacheCreate: 200, SessionCacheRead: 1300, SessionOutput: 800, JudgeModel: "claude-sonnet-4-6", JudgeIn: 60, JudgeOut: 25, FailureReason: "trajectory check: required event \"agent.tool_use\" not seen"},
	}
	var sb strings.Builder
	printSummary(&sb, results)
	out := sb.String()
	for _, want := range []string{
		"L5 scenario summary",
		"alpha", "PASS", "30s",
		"beta", "FAIL", "90s",
		"Totals 1/2",
		"Cost estimate",
		"anthropic.com/pricing",
		"reason: trajectory check",
		// beta has cache activity → the cache breakdown line is printed
		"cache: write=200 read=1300",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n%s", want, out)
		}
	}
}

func TestPrintSummary_flagsUnpriced(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	results := []scenarioResult{
		{Name: "x", Pass: true, Model: "unreleased-model", SessionInput: 10, SessionOutput: 5, JudgeModel: "claude-sonnet-4-6"},
	}
	var sb strings.Builder
	printSummary(&sb, results)
	out := sb.String()
	if !strings.Contains(out, "unreleased-model") {
		t.Errorf("expected unpriced model to be flagged:\n%s", out)
	}
}

// --- judge prompt -------------------------------------------------------

func TestBuildJudgeUserPrompt_truncatesTranscript(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	var events []client.SessionEvent
	for i := 0; i < 30; i++ {
		events = append(events, rawEvent(t, fmt.Sprintf("ev%d", i), "agent.message", fmt.Sprintf(`{"id":"ev%d","type":"agent.message"}`, i)))
	}
	prompt := buildJudgeUserPrompt(&Scenario{Question: "Q", Rubric: "R"}, "answer 55", events)
	if !strings.Contains(prompt, "answer 55") {
		t.Error("prompt missing final answer")
	}
	if !strings.Contains(prompt, "TRANSCRIPT (last 20 events") {
		t.Error("prompt should declare truncation to 20")
	}
}

// --- ${SCENARIO_DIR} substitution ---------------------------------------

func TestLoad_substitutesScenarioDir(t *testing.T) {
	// The loader replaces ${SCENARIO_DIR} with the absolute path to
	// the YAML's directory so resources like
	// claude-managed-agents_skill can reference fixture dirs that
	// travel with the scenario file rather than the cwd of `go test`.
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "withsub.yaml")
	writeFile(t, yamlPath, `
name: withsub
terraform_config: |
  resource "x" "y" {
    source_dir = "${SCENARIO_DIR}/../fixtures/example"
  }
question: q
rubric: r
`)
	s, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if strings.Contains(s.TerraformConfig, "${SCENARIO_DIR}") {
		t.Errorf("expected ${SCENARIO_DIR} to be substituted, got:\n%s", s.TerraformConfig)
	}
	if !strings.Contains(s.TerraformConfig, dir) {
		t.Errorf("expected absolute YAML dir %q in config, got:\n%s", dir, s.TerraformConfig)
	}
}

// --- memory store auto-attach ------------------------------------------

func TestExtractMemoryStoreResources(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	state := &terraform.State{
		Modules: []*terraform.ModuleState{{
			Path: []string{"root"},
			Resources: map[string]*terraform.ResourceState{
				"claude-managed-agents_memory_store.beta": {
					Type:    "claude-managed-agents_memory_store",
					Primary: &terraform.InstanceState{ID: "memstore_02BETA", Attributes: map[string]string{"id": "memstore_02BETA"}},
				},
				"claude-managed-agents_memory_store.alpha": {
					Type:    "claude-managed-agents_memory_store",
					Primary: &terraform.InstanceState{ID: "memstore_01ALPHA", Attributes: map[string]string{"id": "memstore_01ALPHA"}},
				},
				"claude-managed-agents_agent.target": {
					Type:    "claude-managed-agents_agent",
					Primary: &terraform.InstanceState{ID: "agent_01ABC", Attributes: map[string]string{"id": "agent_01ABC"}},
				},
			},
		}},
	}
	got := extractMemoryStoreResources(state)
	if len(got) != 2 {
		t.Fatalf("got %d resources, want 2", len(got))
	}
	// Sorted address order means alpha precedes beta.
	if got[0].Type != "memory_store" || got[0].MemoryStoreID != "memstore_01ALPHA" {
		t.Errorf("got[0] = %+v, want type=memory_store id=memstore_01ALPHA", got[0])
	}
	if got[1].MemoryStoreID != "memstore_02BETA" {
		t.Errorf("got[1] = %+v, want id=memstore_02BETA", got[1])
	}
}

func TestExtractMemoryStoreResources_None(t *testing.T) {
	// When no memory_store resources are declared the harness must
	// return nil (not an empty slice) so JSON serialization omits the
	// resources field via omitempty.
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run scenario harness unit tests")
	}
	state := &terraform.State{
		Modules: []*terraform.ModuleState{{
			Path: []string{"root"},
			Resources: map[string]*terraform.ResourceState{
				"claude-managed-agents_agent.target": {
					Type:    "claude-managed-agents_agent",
					Primary: &terraform.InstanceState{ID: "agent_01ABC", Attributes: map[string]string{"id": "agent_01ABC"}},
				},
			},
		}},
	}
	got := extractMemoryStoreResources(state)
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

// --- helpers ------------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func validYAML(name string) string {
	return fmt.Sprintf(`
name: %s
terraform_config: "resource \"a\" \"b\" {}"
question: q
rubric: r
`, name)
}

func synthEvents(t *testing.T, types ...string) []client.SessionEvent {
	t.Helper()
	out := make([]client.SessionEvent, 0, len(types))
	for i, typ := range types {
		id := fmt.Sprintf("ev%d", i)
		body, _ := json.Marshal(map[string]string{"id": id, "type": typ})
		out = append(out, rawEvent(t, id, typ, string(body)))
	}
	return out
}

func rawEvent(t *testing.T, id, typ, body string) client.SessionEvent {
	t.Helper()
	var ev client.SessionEvent
	if err := json.Unmarshal([]byte(body), &ev); err != nil {
		t.Fatalf("rawEvent unmarshal %q: %v", body, err)
	}
	// UnmarshalJSON populates ID/Type/RawData. Defensive fallback for
	// the rare case the test body intentionally omitted those.
	if ev.ID == "" {
		ev.ID = id
	}
	if ev.Type == "" {
		ev.Type = typ
	}
	return ev
}
