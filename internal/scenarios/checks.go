package scenarios

import (
	"fmt"

	"github.com/andasv/terraform-provider-anthropic-claude-managed-agents/internal/client"
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
