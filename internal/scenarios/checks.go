package scenarios

import (
	"context"
	"fmt"

	"github.com/modus-agendi/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

// TrajectoryCheck is a deterministic predicate over the recorded session
// event trajectory. It returns nil on success or a single-line error
// describing the failure (surfaced verbatim in test output).
type TrajectoryCheck func(events []client.SessionEvent) error

// checkRegistry maps a YAML key to its builder. Builders accept the
// YAML-decoded `any` argument and return a TrajectoryCheck closure (or an
// error if the arg type is wrong). The registry is closed Go-side; new
// check types are added by registering a builder here.
var checkRegistry = map[string]func(arg any) (TrajectoryCheck, error){
	"require_event":                buildRequireEvent,
	"require_terminal_stop_reason": buildRequireTerminalStopReason,
	"require_no_session_errors":    buildRequireNoSessionErrors,
	"require_tool_use_named":       buildRequireToolUseNamed,
	"require_outcome_result":       buildRequireOutcomeResult,
}

// buildRequireEvent: arg must be a non-empty string naming an event type
// (e.g. "agent.tool_use"). The check passes if any event in the
// trajectory has that exact type.
func buildRequireEvent(arg any) (TrajectoryCheck, error) {
	typ, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("require_event: arg must be string, got %T", arg)
	}
	if typ == "" {
		return nil, fmt.Errorf("require_event: arg must be non-empty")
	}
	return func(events []client.SessionEvent) error {
		for _, e := range events {
			if e.Type == typ {
				return nil
			}
		}
		return fmt.Errorf("required event %q not seen", typ)
	}, nil
}

// buildRequireTerminalStopReason: arg must be a non-empty string naming
// the expected stop_reason.type on a session.status_idle event (e.g.
// "end_turn"). Passes if at least one status_idle event carries that
// stop_reason.
func buildRequireTerminalStopReason(arg any) (TrajectoryCheck, error) {
	want, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("require_terminal_stop_reason: arg must be string, got %T", arg)
	}
	if want == "" {
		return nil, fmt.Errorf("require_terminal_stop_reason: arg must be non-empty")
	}
	return func(events []client.SessionEvent) error {
		for _, e := range events {
			if e.Type != "session.status_idle" {
				continue
			}
			got, err := e.StopReasonType()
			if err != nil {
				return fmt.Errorf("parse stop_reason on %s: %w", e.ID, err)
			}
			if got == want {
				return nil
			}
		}
		return fmt.Errorf("no session.status_idle with stop_reason.type=%q", want)
	}, nil
}

// buildRequireNoSessionErrors: arg is ignored (the YAML convention is to
// supply `true` but we accept any value). The check fails if any
// session.error event appears.
func buildRequireNoSessionErrors(_ any) (TrajectoryCheck, error) {
	return func(events []client.SessionEvent) error {
		for _, e := range events {
			if e.Type != "session.error" {
				continue
			}
			msg, err := e.ErrorMessage()
			if err != nil {
				return fmt.Errorf("session.error %s: parse: %w", e.ID, err)
			}
			if msg == "" {
				msg = "(empty)"
			}
			return fmt.Errorf("session error: %s", msg)
		}
		return nil
	}, nil
}

// buildRequireToolUseNamed: arg must be a non-empty string naming the
// tool. Passes if any agent.tool_use / agent.mcp_tool_use /
// agent.custom_tool_use event in the trajectory carries that `name`.
func buildRequireToolUseNamed(arg any) (TrajectoryCheck, error) {
	want, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("require_tool_use_named: arg must be string, got %T", arg)
	}
	if want == "" {
		return nil, fmt.Errorf("require_tool_use_named: arg must be non-empty")
	}
	return func(events []client.SessionEvent) error {
		for _, e := range events {
			switch e.Type {
			case "agent.tool_use", "agent.mcp_tool_use", "agent.custom_tool_use":
				name, err := e.ToolUseName()
				if err != nil {
					return fmt.Errorf("parse tool_use on %s: %w", e.ID, err)
				}
				if name == want {
					return nil
				}
			}
		}
		return fmt.Errorf("no tool_use event named %q", want)
	}, nil
}

// buildRequireOutcomeResult: arg must be a non-empty string naming the
// expected define_outcome verdict (e.g. "satisfied"). Passes if any
// span.outcome_evaluation_end event in the trajectory carries that result.
func buildRequireOutcomeResult(arg any) (TrajectoryCheck, error) {
	want, ok := arg.(string)
	if !ok {
		return nil, fmt.Errorf("require_outcome_result: arg must be string, got %T", arg)
	}
	if want == "" {
		return nil, fmt.Errorf("require_outcome_result: arg must be non-empty")
	}
	return func(events []client.SessionEvent) error {
		for _, e := range events {
			if e.Type != "span.outcome_evaluation_end" {
				continue
			}
			got, err := e.OutcomeResult()
			if err != nil {
				return fmt.Errorf("parse outcome_evaluation_end on %s: %w", e.ID, err)
			}
			if got == want {
				return nil
			}
		}
		return fmt.Errorf("no span.outcome_evaluation_end with result=%q", want)
	}, nil
}

// buildAll resolves every CheckSpec on a scenario to its concrete
// TrajectoryCheck. Returns checks in declaration order so failures point
// at the YAML entry that produced them. Assumes validate() has already
// run; unknown keys panic-style return error here for defense in depth.
func buildAll(specs []CheckSpec) ([]TrajectoryCheck, error) {
	out := make([]TrajectoryCheck, 0, len(specs))
	for i, spec := range specs {
		for key, arg := range spec {
			builder, ok := checkRegistry[key]
			if !ok {
				return nil, fmt.Errorf("trajectory_checks[%d]: unknown check %q", i, key)
			}
			c, err := builder(arg)
			if err != nil {
				return nil, fmt.Errorf("trajectory_checks[%d] %q: %w", i, key, err)
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// --- deployment-run checks (deployment-kind scenarios) -----------------------

// RunCheck is a deterministic predicate over the DeploymentRun produced by a
// manual trigger (POST /v1/deployments/{id}/run). It asserts properties of the
// run record itself (session linkage, trigger type, typed error) rather than
// the session event stream.
type RunCheck func(run *client.DeploymentRun) error

var runCheckRegistry = map[string]func(arg any) (RunCheck, error){
	"require_run_trigger_type": buildRequireRunTriggerType,
	"require_run_session_set":  buildRequireRunSessionSet,
	"require_run_no_error":     buildRequireRunNoError,
	"require_run_error_type":   buildRequireRunErrorType,
}

// buildRequireRunTriggerType: arg is the expected trigger_context.type
// ("manual" | "schedule").
func buildRequireRunTriggerType(arg any) (RunCheck, error) {
	want, ok := arg.(string)
	if !ok || want == "" {
		return nil, fmt.Errorf("require_run_trigger_type: arg must be a non-empty string, got %T", arg)
	}
	return func(run *client.DeploymentRun) error {
		if run.TriggerContext.Type != want {
			return fmt.Errorf("run trigger_context.type=%q, want %q", run.TriggerContext.Type, want)
		}
		return nil
	}, nil
}

// buildRequireRunSessionSet: arg ignored. Passes if the run created a session.
func buildRequireRunSessionSet(_ any) (RunCheck, error) {
	return func(run *client.DeploymentRun) error {
		if run.SessionID == nil || *run.SessionID == "" {
			return fmt.Errorf("run has no session_id (run did not start a session)")
		}
		return nil
	}, nil
}

// buildRequireRunNoError: arg ignored. Passes if the run carries no error.
func buildRequireRunNoError(_ any) (RunCheck, error) {
	return func(run *client.DeploymentRun) error {
		if run.Error != nil {
			return fmt.Errorf("run failed: %s (%s)", run.Error.Type, run.Error.Message)
		}
		return nil
	}, nil
}

// buildRequireRunErrorType: arg is the expected run error.type (e.g.
// "environment_archived_error").
func buildRequireRunErrorType(arg any) (RunCheck, error) {
	want, ok := arg.(string)
	if !ok || want == "" {
		return nil, fmt.Errorf("require_run_error_type: arg must be a non-empty string, got %T", arg)
	}
	return func(run *client.DeploymentRun) error {
		if run.Error == nil {
			return fmt.Errorf("run succeeded, expected error.type=%q", want)
		}
		if run.Error.Type != want {
			return fmt.Errorf("run error.type=%q, want %q", run.Error.Type, want)
		}
		return nil
	}, nil
}

func buildAllRunChecks(specs []CheckSpec) ([]RunCheck, error) {
	out := make([]RunCheck, 0, len(specs))
	for i, spec := range specs {
		for key, arg := range spec {
			builder, ok := runCheckRegistry[key]
			if !ok {
				return nil, fmt.Errorf("run_checks[%d]: unknown check %q", i, key)
			}
			c, err := builder(arg)
			if err != nil {
				return nil, fmt.Errorf("run_checks[%d] %q: %w", i, key, err)
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// --- lifecycle checks (lifecycle-kind scenarios) -----------------------------

// LifecycleCheck drives deployment lifecycle operations against the live API
// and asserts the resulting state transitions. Unlike the other check
// families it makes API calls itself (no session/judge involved).
type LifecycleCheck func(ctx context.Context, c *client.Client, deploymentID string) error

var lifecycleCheckRegistry = map[string]func(arg any) (LifecycleCheck, error){
	"require_pause_resume_cycle": buildRequirePauseResumeCycle,
}

// buildRequirePauseResumeCycle: arg ignored. Pauses then resumes the
// deployment, asserting status=paused + paused_reason.type=manual on pause and
// status=active + paused_reason=null on resume.
func buildRequirePauseResumeCycle(_ any) (LifecycleCheck, error) {
	return func(ctx context.Context, c *client.Client, deploymentID string) error {
		paused, err := c.PauseDeployment(ctx, deploymentID)
		if err != nil {
			return fmt.Errorf("pause: %w", err)
		}
		if paused.Status != "paused" {
			return fmt.Errorf("after pause: status=%q, want paused", paused.Status)
		}
		if paused.PausedReason == nil || paused.PausedReason.Type != "manual" {
			return fmt.Errorf("after pause: paused_reason=%+v, want type=manual", paused.PausedReason)
		}
		resumed, err := c.ResumeDeployment(ctx, deploymentID)
		if err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		if resumed.Status != "active" {
			return fmt.Errorf("after resume: status=%q, want active", resumed.Status)
		}
		if resumed.PausedReason != nil {
			return fmt.Errorf("after resume: paused_reason=%+v, want null", resumed.PausedReason)
		}
		return nil
	}, nil
}

func buildAllLifecycleChecks(specs []CheckSpec) ([]LifecycleCheck, error) {
	out := make([]LifecycleCheck, 0, len(specs))
	for i, spec := range specs {
		for key, arg := range spec {
			builder, ok := lifecycleCheckRegistry[key]
			if !ok {
				return nil, fmt.Errorf("lifecycle_checks[%d]: unknown check %q", i, key)
			}
			c, err := builder(arg)
			if err != nil {
				return nil, fmt.Errorf("lifecycle_checks[%d] %q: %w", i, key, err)
			}
			out = append(out, c)
		}
	}
	return out, nil
}
