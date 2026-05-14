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
// Cache creation / read tokens are not separately priced — they fold
// into input tokens at the same per-Mtok rate as a worst-case estimate.

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

// estimateUSD computes the dollar cost of an in/out token pair for the
// given model. Unknown models contribute zero (the cost summary line
// flags unpriced models explicitly).
func estimateUSD(model string, in, out int) float64 {
	p, ok := pricing[model]
	if !ok {
		return 0
	}
	const mtok = 1_000_000.0
	return (float64(in)/mtok)*p.InputPerMtok + (float64(out)/mtok)*p.OutputPerMtok
}

// isPriced reports whether the model has a row in the pricing table.
// Used by the cost summary printer to flag missing-model gaps.
func isPriced(model string) bool {
	_, ok := pricing[model]
	return ok
}
