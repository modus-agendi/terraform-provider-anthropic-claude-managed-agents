// Package scenarios is the L5 behavioral test layer.
//
// Scenarios are YAML files under internal/scenarios/scenarios/ describing
// a Terraform config + a question for the agent to answer. The harness
// applies the config, opens a real session against api.anthropic.com,
// captures the full event trajectory, runs deterministic checks, then
// asks a separate "judge" model (via /v1/messages) to grade the final
// answer against a rubric. L5 is gated by TF_ACC_SCENARIOS=1 — it bills
// real inference tokens — and runs nightly + as a release gate.
//
// See README.md and CLAUDE.md "Test layer model" for the broader
// rationale.
package scenarios

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
	"github.com/andasv/terraform-provider-claude-managed-agents/internal/provider"
)

// pollInterval is how often the harness re-polls /v1/sessions/{id}/events
// while waiting for the agent to finish. 2s is documented in the plan as
// adequate; lower would beat on the API without practical benefit.
const pollInterval = 2 * time.Second

// targetResourceAddress is the fixed Terraform address every scenario
// must use for the agent under test. Documented in README.md.
const targetResourceAddress = "claude-managed-agents_agent.target"

// envResourceAddress is optional. If declared in the scenario, the
// harness wires it to the session's environment_id.
const envResourceAddress = "claude-managed-agents_environment.sandbox"

// judgeSystemPrompt is the shared judge instruction. Reused across every
// scenario so a single prompt change improves grading consistency
// project-wide.
const judgeSystemPrompt = `You are an evaluator for AI agent test scenarios. Given a rubric, a task description, and the agent's final response, decide whether the agent satisfied the rubric. Respond with a single JSON object — no prose.

Schema: {"verdict": "PASS" | "FAIL", "reason": <one-line string>}`

// scenarioResult is one row of the cost-summary table printed after the
// full TestScenarios run completes.
type scenarioResult struct {
	Name        string
	Pass        bool
	DurationSec int
	Model       string // agent model (looked up from agent state)
	// SessionInput / CacheCreate / CacheRead are the three input-side
	// token buckets from SessionUsage. They are priced separately so the
	// cost estimate reflects the 90% discount on cache reads.
	SessionInput       int
	SessionCacheCreate int
	SessionCacheRead   int
	SessionOutput      int
	JudgeModel         string
	JudgeIn            int
	JudgeOut           int
	FailureReason      string // populated when Pass == false; one-line summary
}

// sessionInTotal returns the displayed input total (regular + cache
// create + cache read). Cost estimation prices each bucket separately
// via estimateUSD; this helper is only for the CI summary line.
func (r scenarioResult) sessionInTotal() int {
	return r.SessionInput + r.SessionCacheCreate + r.SessionCacheRead
}

// aggregator accumulates per-scenario results across the t.Run subtests.
// Not safe for concurrent use — t.Run calls are serialized inside
// TestScenarios.
type aggregator struct {
	results []scenarioResult
}

// runScenario is the per-scenario test body. It builds a TestCase whose
// single Step extracts the agent id via a Check closure, then drives the
// session loop + judge from the same closure. Terraform destroy + the
// deferred ArchiveSession handle cleanup even on failure.
//
// Why one big closure inside a Check rather than running the session
// outside the test case: resource.UnitTest tears down state at function
// return; the agent only exists between Step.Check and the implicit
// Destroy step. Doing the session work inside Check guarantees the
// resource is alive while the session runs.
func runScenario(t *testing.T, scn *Scenario, agg *aggregator) {
	t.Helper()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Fatal("ANTHROPIC_API_KEY is required for L5 scenarios")
	}

	checks, err := buildAll(scn.TrajectoryChecks)
	if err != nil {
		t.Fatalf("scenarios: build checks: %v", err)
	}

	c, err := client.New(client.Config{
		APIKey:    apiKey,
		UserAgent: "terraform-provider-claude-managed-agents/scenarios",
	})
	if err != nil {
		t.Fatalf("scenarios: build client: %v", err)
	}

	// Result row populated as the scenario runs; recorded on the
	// aggregator at function end regardless of pass/fail.
	result := scenarioResult{
		Name:       scn.Name,
		JudgeModel: scn.JudgeModel,
	}
	start := time.Now()
	defer func() {
		result.DurationSec = int(time.Since(start).Round(time.Second).Seconds())
		agg.results = append(agg.results, result)
	}()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{
				Config: scn.TerraformConfig,
				Check: func(s *terraform.State) error {
					agentID, agentModel, err := extractAgent(s)
					if err != nil {
						return err
					}
					result.Model = agentModel
					envID, _ := extractEnvironment(s) // optional

					ctx, cancel := context.WithTimeout(context.Background(), scn.Timeout)
					defer cancel()

					sess, err := c.CreateSession(ctx, client.SessionCreateRequest{
						AgentID:       agentID,
						EnvironmentID: envID,
						Title:         "tf-acc-test-scenarios-" + scn.Name,
					})
					if err != nil {
						result.FailureReason = "create session: " + err.Error()
						return fmt.Errorf("create session: %w", err)
					}
					// Always archive on exit. Best-effort — sweeper
					// cleans up if this fails.
					defer func() {
						// Use a fresh, short context — the parent may
						// already be cancelled if we timed out.
						archCtx, archCancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer archCancel()
						_ = c.ArchiveSession(archCtx, sess.ID)
					}()

					if err := c.PostSessionEvents(ctx, sess.ID, []client.UserEvent{{
						Type: "user.message",
						Content: []client.EventContent{{
							Type: "text",
							Text: scn.Question,
						}},
					}}); err != nil {
						result.FailureReason = "post user event: " + err.Error()
						return fmt.Errorf("post user event: %w", err)
					}

					trajectory, pollErr := pollUntilTerminal(ctx, c, sess.ID)
					// Capture usage even on poll failure (final
					// GetSession may report partial usage).
					if final, gerr := c.GetSession(context.Background(), sess.ID); gerr == nil && final.Usage != nil {
						result.SessionInput = final.Usage.InputTokens
						result.SessionCacheCreate = final.Usage.CacheCreationInputTokens
						result.SessionCacheRead = final.Usage.CacheReadInputTokens
						result.SessionOutput = final.Usage.OutputTokens
					}
					if pollErr != nil {
						result.FailureReason = "session loop: " + pollErr.Error()
						return fmt.Errorf("session loop: %w", pollErr)
					}

					// Trajectory checks: run them all, surface the
					// first failure but record every one in test
					// output for debugging.
					var checkErrs []string
					for _, chk := range checks {
						if err := chk(trajectory); err != nil {
							checkErrs = append(checkErrs, err.Error())
						}
					}

					// Extract final answer from the last agent.message
					// event. Empty is acceptable (judge gets a clear
					// note in the prompt and decides on PASS/FAIL).
					finalAnswer := lastAgentMessage(trajectory)

					verdict, err := c.JudgeVerdict(ctx, client.JudgeRequest{
						Model:        scn.JudgeModel,
						SystemPrompt: judgeSystemPrompt,
						UserPrompt:   buildJudgeUserPrompt(scn, finalAnswer, trajectory),
						MaxTokens:    512,
					})
					if err != nil {
						result.FailureReason = "judge call: " + err.Error()
						return fmt.Errorf("judge call: %w", err)
					}
					// Capture judge token usage from the Messages API
					// response. JudgeResult.Usage is nil only if the
					// upstream API ever omits the usage block (extremely
					// unlikely upstream regression).
					if verdict.Usage != nil {
						result.JudgeIn = verdict.Usage.InputTokens
						result.JudgeOut = verdict.Usage.OutputTokens
					}

					if len(checkErrs) > 0 {
						result.FailureReason = "trajectory check: " + strings.Join(checkErrs, "; ")
						return fmt.Errorf("trajectory check failures: %s", strings.Join(checkErrs, "; "))
					}
					if verdict.Verdict != "PASS" {
						result.FailureReason = "judge FAIL: " + verdict.Reason
						return fmt.Errorf("judge FAIL: %s", verdict.Reason)
					}

					result.Pass = true
					t.Logf("scenario %s PASS: %s", scn.Name, verdict.Reason)
					return nil
				},
			},
		},
	})
}

// pollUntilTerminal repeatedly calls ListSessionEvents until the session
// reaches a terminal state. Returns the full ordered event trajectory.
// Error paths:
//   - session.status_terminated     → "session terminated"
//   - session.error                 → wrapped ErrorMessage()
//   - context deadline / cancel     → ctx.Err()
//   - transport / API error         → propagated
//
// session.status_idle is treated as success when its stop_reason.type is
// "end_turn"; other stop_reasons (e.g. requires_action awaiting a tool
// approval) continue polling — the harness assumes auto-allow tools.
func pollUntilTerminal(ctx context.Context, c *client.Client, sessionID string) ([]client.SessionEvent, error) {
	var (
		trajectory   []client.SessionEvent
		seen         = map[string]bool{}
		lastTS       time.Time
	)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return trajectory, ctx.Err()
		default:
		}
		// Re-fetch with a small overlap window. created_at[gt] is
		// exclusive, so we step back ~1s to avoid losing events that
		// share a timestamp with the previous max. The seen-ID set
		// drops the duplicates this introduces.
		var cursor time.Time
		if !lastTS.IsZero() {
			cursor = lastTS.Add(-time.Second)
		}
		page, err := c.ListSessionEvents(ctx, sessionID, client.ListSessionEventsParams{
			CreatedAfter: cursor,
		})
		if err != nil {
			return trajectory, err
		}
		for _, e := range page.Data {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			trajectory = append(trajectory, e)
			if e.ProcessedAt != nil && e.ProcessedAt.After(lastTS) {
				lastTS = *e.ProcessedAt
			}
			switch e.Type {
			case "session.status_terminated":
				return trajectory, fmt.Errorf("session terminated")
			case "session.error":
				msg, _ := e.ErrorMessage()
				if msg == "" {
					msg = "(unknown)"
				}
				return trajectory, fmt.Errorf("session error: %s", msg)
			case "session.status_idle":
				reason, _ := e.StopReasonType()
				if reason == "end_turn" {
					return trajectory, nil
				}
				// Other stop reasons (e.g. requires_action) — keep
				// polling. With auto-allow tools the agent should
				// transition through these without test intervention.
			}
		}
		// No new events yet; wait for the next tick before polling
		// again. Honour cancellation in the wait too.
		select {
		case <-ctx.Done():
			return trajectory, ctx.Err()
		case <-ticker.C:
		}
	}
}

// lastAgentMessage concatenates the text content of the latest
// agent.message in the trajectory. Returns the empty string if no
// agent.message exists (the judge prompt makes this explicit so the
// judge can return FAIL with a clear reason).
func lastAgentMessage(events []client.SessionEvent) string {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type != "agent.message" {
			continue
		}
		text, err := events[i].AgentMessageText()
		if err != nil {
			return ""
		}
		return text
	}
	return ""
}

// buildJudgeUserPrompt renders the rubric-grading template. Truncates the
// transcript to the last 20 events so the judge call stays under a few
// hundred input tokens.
func buildJudgeUserPrompt(scn *Scenario, finalAnswer string, events []client.SessionEvent) string {
	const maxTranscript = 20
	tail := events
	if len(tail) > maxTranscript {
		tail = tail[len(tail)-maxTranscript:]
	}
	var transcript strings.Builder
	for _, e := range tail {
		transcript.WriteString(e.Type)
		transcript.WriteString("\n")
	}
	if finalAnswer == "" {
		finalAnswer = "(agent produced no final text response)"
	}
	return fmt.Sprintf(`TASK: %s
RUBRIC: %s
AGENT FINAL RESPONSE: %s
TRANSCRIPT (last %d events, types only):
%s
Did the agent satisfy the rubric? Reply as JSON only.`,
		strings.TrimSpace(scn.Question),
		strings.TrimSpace(scn.Rubric),
		strings.TrimSpace(finalAnswer),
		len(tail),
		transcript.String(),
	)
}

// extractAgent pulls the id and model from the
// claude-managed-agents_agent.target resource in the post-apply state.
// The id is returned as-is; model is the underlying string id (not the
// nested ModelConfig).
func extractAgent(s *terraform.State) (id, model string, err error) {
	rs, ok := s.RootModule().Resources[targetResourceAddress]
	if !ok {
		return "", "", fmt.Errorf("scenarios: expected resource %q in state (every scenario must declare it)", targetResourceAddress)
	}
	id = rs.Primary.Attributes["id"]
	if id == "" {
		return "", "", fmt.Errorf("scenarios: %s has no id in state", targetResourceAddress)
	}
	model = rs.Primary.Attributes["model"]
	return id, model, nil
}

// extractEnvironment returns the optional environment id and whether the
// env resource was found. Missing env returns ("", nil) — callers treat
// it as "no environment_id on the session", which the upstream API
// requires only when a sandboxed tool runs.
func extractEnvironment(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources[envResourceAddress]
	if !ok {
		return "", nil
	}
	id := rs.Primary.Attributes["id"]
	return id, nil
}

// protoFactories builds the provider factory map for the scenarios
// harness. Reuses the real provider.New, so resource behavior matches
// what users get from the registry.
func protoFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"claude-managed-agents": providerserver.NewProtocol6WithError(provider.New("scenarios", "scenarios")()),
	}
}

// printSummary writes the cost-summary table to w. Format matches the
// example in the plan; widths are loose because we don't ship many
// scenarios per run.
func printSummary(w *strings.Builder, results []scenarioResult) {
	w.WriteString("======================================================================\n")
	w.WriteString("L5 scenario summary\n")
	w.WriteString("----------------------------------------------------------------------\n")
	var (
		totalSessIn, totalSessOut   int
		totalJudgeIn, totalJudgeOut int
		passes                      int
		totalCost                   float64
		unpriced                    []string
	)
	for _, r := range results {
		status := "FAIL"
		if r.Pass {
			status = "PASS"
			passes++
		}
		sessIn := r.sessionInTotal()
		fmt.Fprintf(w, "%-40s %s    %3ds    in=%-6d out=%-6d  judge: in=%-4d out=%-4d\n",
			r.Name, status, r.DurationSec,
			sessIn, r.SessionOutput,
			r.JudgeIn, r.JudgeOut,
		)
		// Surface the cache breakdown when caching contributed — makes
		// the cost line's discount visible at a glance.
		if r.SessionCacheCreate > 0 || r.SessionCacheRead > 0 {
			fmt.Fprintf(w, "  cache: write=%d read=%d (read priced at %.0f%% of input)\n",
				r.SessionCacheCreate, r.SessionCacheRead, cacheReadMultiplier*100)
		}
		if !r.Pass && r.FailureReason != "" {
			fmt.Fprintf(w, "  reason: %s\n", r.FailureReason)
		}
		totalSessIn += sessIn
		totalSessOut += r.SessionOutput
		totalJudgeIn += r.JudgeIn
		totalJudgeOut += r.JudgeOut
		totalCost += estimateUSD(r.Model, r.SessionInput, r.SessionCacheCreate, r.SessionCacheRead, r.SessionOutput)
		totalCost += estimateUSD(r.JudgeModel, r.JudgeIn, 0, 0, r.JudgeOut)
		if r.Model != "" && !isPriced(r.Model) {
			unpriced = append(unpriced, r.Model)
		}
		if r.JudgeModel != "" && !isPriced(r.JudgeModel) {
			unpriced = append(unpriced, r.JudgeModel)
		}
	}
	w.WriteString("----------------------------------------------------------------------\n")
	fmt.Fprintf(w, "Totals %d/%d                                in=%-6d out=%-6d  judge: in=%-4d out=%-4d\n",
		passes, len(results), totalSessIn, totalSessOut, totalJudgeIn, totalJudgeOut)
	fmt.Fprintf(w, "Cost estimate: ~$%0.2f\n", totalCost)
	w.WriteString("        See https://www.anthropic.com/pricing for current rates.\n")
	if len(unpriced) > 0 {
		fmt.Fprintf(w, "        NOTE: unpriced models contributed $0 to the estimate: %s\n", strings.Join(uniq(unpriced), ", "))
	}
	w.WriteString("======================================================================\n")
}

// uniq returns the input slice with duplicates removed, order-preserving.
func uniq(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
