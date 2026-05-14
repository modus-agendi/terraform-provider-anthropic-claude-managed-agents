package scenarios

// Source of truth: https://www.anthropic.com/pricing
// Last verified: 2026-05-14
//
// These values drift whenever Anthropic changes pricing. The table is a
// rough lower-bound estimate for operational signal in CI output — NOT
// a billing source. The CI summary line includes the pricing-page URL
// so reviewers can sanity-check the estimate against the current rate.
//
// Tiered pricing (Opus discounts above 200k context tokens) is not modeled;
// scenarios are small turns where the discount tier doesn't trigger.
// Cache pricing relative to the base input rate:
//
//   - cache_creation_input_tokens → 1.25× (5-minute TTL, the API default)
//   - cache_read_input_tokens     → 0.10× (90% off)
//
// The 1-hour cache write tier (2× input) is not modeled — the upstream
// usage block does not break it out, and the API default is 5-minute.

const (
	cacheCreateMultiplier = 1.25
	cacheReadMultiplier   = 0.10
)

// Pricing is one row of the per-Mtok rate table. InputPerMtok / OutputPerMtok
// are denominated in USD per million tokens.
type Pricing struct {
	InputPerMtok  float64
	OutputPerMtok float64
}

// pricing keys are model ids as they appear in:
//
//   - Session.Model / agent.model.id (e.g. "claude-opus-4-7")
//   - Judge requests (e.g. "claude-sonnet-4-6")
//
// Models absent from the table contribute zero to the cost estimate
// (no panic, just under-counting). The summary printer flags
// any-unpriced-model with a stderr note so the gap is auditable.
var pricing = map[string]Pricing{
	"claude-opus-4-7":           {InputPerMtok: 15.00, OutputPerMtok: 75.00},
	"claude-sonnet-4-6":         {InputPerMtok: 3.00, OutputPerMtok: 15.00},
	"claude-haiku-4-5-20251001": {InputPerMtok: 0.80, OutputPerMtok: 4.00},
}

// estimateUSD computes the dollar cost from a token breakdown for the
// given model. cacheCreate / cacheRead are billed at the input rate
// times cacheCreateMultiplier / cacheReadMultiplier respectively;
// callers that don't track caching (e.g. the judge call) pass zero for
// both. Unknown models contribute zero (the cost summary line flags
// unpriced models explicitly).
func estimateUSD(model string, in, cacheCreate, cacheRead, out int) float64 {
	p, ok := pricing[model]
	if !ok {
		return 0
	}
	const mtok = 1_000_000.0
	return (float64(in)/mtok)*p.InputPerMtok +
		(float64(cacheCreate)/mtok)*p.InputPerMtok*cacheCreateMultiplier +
		(float64(cacheRead)/mtok)*p.InputPerMtok*cacheReadMultiplier +
		(float64(out)/mtok)*p.OutputPerMtok
}

// isPriced reports whether the model has a row in the pricing table.
// Used by the cost summary printer to flag missing-model gaps.
func isPriced(model string) bool {
	_, ok := pricing[model]
	return ok
}
