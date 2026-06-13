package scenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// defaultTimeout is applied when a scenario YAML omits `timeout` or sets it
// to zero. Five minutes is well above the observed p99 turn duration for
// the toolset scenarios we ship today; tune downward only if a specific
// scenario regularly times out earlier.
const defaultTimeout = 5 * time.Minute

// defaultJudgeModel is used when a scenario doesn't override judge_model.
// Kept in sync with the client.JudgeVerdict default — re-declared here so
// the harness has its own knob without importing internal/client just for
// a string constant.
const defaultJudgeModel = "claude-sonnet-4-6"

// Scenario is the in-memory representation of one YAML file under
// internal/scenarios/scenarios/.
//
// Required fields: Name, TerraformConfig, Question, Rubric. Validation
// happens at Load time; an invalid scenario returns a clear error rather
// than silently degrading to a half-formed test.
type Scenario struct {
	// Name is the t.Run subtest name. Must be unique per file (Load
	// surfaces this via the file path, not by scanning).
	Name string `yaml:"name"`
	// Timeout is the wall-clock cap for the whole scenario: provider
	// apply, session loop, judge call combined. Defaults to 5min when
	// absent or zero.
	Timeout time.Duration `yaml:"timeout"`
	// TerraformConfig is the HCL fragment applied via resource.UnitTest.
	// The harness wraps it with the standard test provider factories;
	// configs must declare exactly one `claude-managed-agents_agent.target`
	// resource that the harness can address.
	TerraformConfig string `yaml:"terraform_config"`
	// Question is the user prompt sent as the first session event.
	Question string `yaml:"question"`
	// Rubric is fed verbatim to the judge model alongside the agent's
	// final answer.
	Rubric string `yaml:"rubric"`
	// JudgeModel overrides the default judge model. Empty means use
	// defaultJudgeModel.
	JudgeModel string `yaml:"judge_model"`
	// TrajectoryChecks are deterministic event-stream assertions run
	// before the judge call. Each entry is a single-key map keyed by a
	// registered check name (see checks.go).
	TrajectoryChecks []CheckSpec `yaml:"trajectory_checks"`

	// Kind selects the scenario shape: "agent" (default — open a session
	// against claude-managed-agents_agent.target), "deployment" (fire
	// claude-managed-agents_deployment.target via a manual run and observe
	// the resulting session + run record), or "lifecycle" (drive deployment
	// pause/resume client ops; no session/judge).
	Kind string `yaml:"kind"`
	// OutcomeDescription is the judge's TASK line for deployment scenarios
	// whose session is driven by the deployment's own initial_events (so
	// there is no `question`). Ignored for agent scenarios.
	OutcomeDescription string `yaml:"outcome_description"`
	// RunChecks assert over the DeploymentRun produced by a manual trigger
	// (deployment kind only). Single-key maps keyed by a registered
	// run-check name (see checks.go runCheckRegistry).
	RunChecks []CheckSpec `yaml:"run_checks"`
	// LifecycleChecks drive + assert deployment lifecycle ops (lifecycle
	// kind only). Single-key maps keyed by a registered lifecycle-check name.
	LifecycleChecks []CheckSpec `yaml:"lifecycle_checks"`
	// PreRunArchive, when set on a deployment scenario, names a deployment
	// dependency to archive out-of-band BEFORE the manual trigger, to drive
	// a run-time error path. Currently only "environment" is supported.
	PreRunArchive string `yaml:"pre_run_archive"`

	// sourcePath is the on-disk path the scenario was loaded from. Set
	// by Load; useful for error messages and test labels.
	sourcePath string
}

// CheckSpec is a single trajectory check declaration. It is structurally
// a one-key map (e.g. `{"require_event": "agent.tool_use"}`); the key
// names the registered check factory and the value is its argument.
type CheckSpec map[string]any

// SourcePath returns the on-disk YAML path the scenario was loaded from.
// Read-only — set by Load.
func (s *Scenario) SourcePath() string { return s.sourcePath }

// Load parses a single YAML file into a Scenario and validates required
// fields + trajectory-check shapes. The caller gets back a fully-realized
// scenario whose checks have been pre-built (so a malformed check fails
// the whole scenario load, not the test runtime).
func Load(path string) (*Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenarios.Load %s: %w", path, err)
	}
	var s Scenario
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("scenarios.Load %s: yaml unmarshal: %w", path, err)
	}
	s.sourcePath = path
	if s.Kind == "" {
		s.Kind = "agent"
	}
	// Substitute ${SCENARIO_DIR} → absolute path to the YAML's
	// directory so resources like claude-managed-agents_skill can
	// reference fixture dirs portably across machines. Done before
	// validate so any escape from substitution is still rejected by
	// the validator.
	if abs, err := filepath.Abs(filepath.Dir(path)); err == nil {
		s.TerraformConfig = strings.ReplaceAll(s.TerraformConfig, "${SCENARIO_DIR}", abs)
	}
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("scenarios.Load %s: %w", path, err)
	}
	if s.Timeout <= 0 {
		s.Timeout = defaultTimeout
	}
	if s.JudgeModel == "" {
		s.JudgeModel = defaultJudgeModel
	}
	return &s, nil
}

// LoadDir walks dir non-recursively, loading every *.yaml / *.yml file.
// Files are returned in lexical order so test output is deterministic.
// Any single file failing to load aborts the whole batch (loud failure
// over silent skip — we want a misspelled check key to break CI, not
// quietly drop a scenario).
func LoadDir(dir string) ([]*Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scenarios.LoadDir %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !hasYAMLExt(name) {
			continue
		}
		paths = append(paths, dir+"/"+name)
	}
	sort.Strings(paths)
	out := make([]*Scenario, 0, len(paths))
	for _, p := range paths {
		s, err := Load(p)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func hasYAMLExt(name string) bool {
	for _, ext := range []string{".yaml", ".yml"} {
		if len(name) > len(ext) && name[len(name)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// validate enforces required-field presence + trajectory-check shape.
// Each trajectory check is built via the registry so a typo in the key
// or a wrong-typed arg is caught at load, before any Terraform applies.
func (s *Scenario) validate() error {
	if s.Name == "" {
		return fmt.Errorf("missing required field: name")
	}
	if s.TerraformConfig == "" {
		return fmt.Errorf("missing required field: terraform_config")
	}

	switch s.Kind {
	case "agent":
		if s.Question == "" {
			return fmt.Errorf("missing required field: question")
		}
		if s.Rubric == "" {
			return fmt.Errorf("missing required field: rubric")
		}
	case "deployment":
		// question / outcome_description / rubric are optional: an
		// error-path scenario fires a run that never starts a session, so
		// it has nothing to judge. But a scenario WITH a rubric needs a
		// TASK line for the judge.
		if s.Rubric != "" && s.Question == "" && s.OutcomeDescription == "" {
			return fmt.Errorf("deployment scenario with a rubric must set question or outcome_description (the judge TASK)")
		}
		if s.PreRunArchive != "" && s.PreRunArchive != "environment" {
			return fmt.Errorf("pre_run_archive: only \"environment\" is supported, got %q", s.PreRunArchive)
		}
		if err := buildAndDiscardRunChecks(s.RunChecks); err != nil {
			return err
		}
	case "lifecycle":
		if len(s.LifecycleChecks) == 0 {
			return fmt.Errorf("lifecycle scenario requires at least one lifecycle_checks entry")
		}
		if s.Question != "" || s.Rubric != "" {
			return fmt.Errorf("lifecycle scenario must not set question/rubric (no session or judge)")
		}
		if err := buildAndDiscardLifecycleChecks(s.LifecycleChecks); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown kind %q (valid: agent, deployment, lifecycle)", s.Kind)
	}

	if s.Kind != "deployment" && len(s.RunChecks) > 0 {
		return fmt.Errorf("run_checks are only valid on deployment scenarios")
	}
	if s.Kind != "lifecycle" && len(s.LifecycleChecks) > 0 {
		return fmt.Errorf("lifecycle_checks are only valid on lifecycle scenarios")
	}

	// trajectory_checks apply to agent + deployment kinds (lifecycle has no
	// session).
	for i, spec := range s.TrajectoryChecks {
		if len(spec) != 1 {
			return fmt.Errorf("trajectory_checks[%d]: expected single-key map, got %d keys", i, len(spec))
		}
		for key, arg := range spec {
			builder, ok := checkRegistry[key]
			if !ok {
				return fmt.Errorf("trajectory_checks[%d]: unknown check %q (valid: %s)", i, key, validCheckKeys())
			}
			if _, err := builder(arg); err != nil {
				return fmt.Errorf("trajectory_checks[%d] %q: %w", i, key, err)
			}
		}
	}
	return nil
}

// buildAndDiscardRunChecks validates run_checks shape + arg types at load.
func buildAndDiscardRunChecks(specs []CheckSpec) error {
	for i, spec := range specs {
		if len(spec) != 1 {
			return fmt.Errorf("run_checks[%d]: expected single-key map, got %d keys", i, len(spec))
		}
		for key, arg := range spec {
			builder, ok := runCheckRegistry[key]
			if !ok {
				return fmt.Errorf("run_checks[%d]: unknown check %q (valid: %s)", i, key, validRunCheckKeys())
			}
			if _, err := builder(arg); err != nil {
				return fmt.Errorf("run_checks[%d] %q: %w", i, key, err)
			}
		}
	}
	return nil
}

// buildAndDiscardLifecycleChecks validates lifecycle_checks at load.
func buildAndDiscardLifecycleChecks(specs []CheckSpec) error {
	for i, spec := range specs {
		if len(spec) != 1 {
			return fmt.Errorf("lifecycle_checks[%d]: expected single-key map, got %d keys", i, len(spec))
		}
		for key, arg := range spec {
			builder, ok := lifecycleCheckRegistry[key]
			if !ok {
				return fmt.Errorf("lifecycle_checks[%d]: unknown check %q (valid: %s)", i, key, validLifecycleCheckKeys())
			}
			if _, err := builder(arg); err != nil {
				return fmt.Errorf("lifecycle_checks[%d] %q: %w", i, key, err)
			}
		}
	}
	return nil
}

// validCheckKeys returns the sorted list of registered trajectory-check
// names, used in load-time error messages so a typo points at the valid set.
func validCheckKeys() string {
	keys := make([]string, 0, len(checkRegistry))
	for k := range checkRegistry {
		keys = append(keys, k)
	}
	return joinSorted(keys)
}

func validRunCheckKeys() string {
	keys := make([]string, 0, len(runCheckRegistry))
	for k := range runCheckRegistry {
		keys = append(keys, k)
	}
	return joinSorted(keys)
}

func validLifecycleCheckKeys() string {
	keys := make([]string, 0, len(lifecycleCheckRegistry))
	for k := range lifecycleCheckRegistry {
		keys = append(keys, k)
	}
	return joinSorted(keys)
}

func joinSorted(keys []string) string {
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}
