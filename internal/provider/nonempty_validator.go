package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Why these exist: the upstream API normalizes an explicit empty collection
// (`foo = []` / `foo = {}`) to null/absent. Terraform core forbids a provider
// from rewriting a known config value to null at plan time, so an explicit
// empty value would otherwise surface as a cryptic "Provider produced
// inconsistent result after apply" crash (issue #79). Rejecting the empty
// value at validate time turns that crash into a clear, actionable error:
// omit the attribute to leave it unset.

const nonEmptyListDetail = "An explicit empty list ([]) is not allowed here. Omit the attribute entirely to leave it unset, or provide at least one element."

const nonEmptyMapDetail = "An explicit empty map ({}) is not allowed here. Omit the attribute entirely to leave it unset, or provide at least one entry."

// nonEmptyListValidator rejects a known, zero-length list.
type nonEmptyListValidator struct{}

// nonEmptyList returns a validator that rejects an explicit empty list.
func nonEmptyList() validator.List { return nonEmptyListValidator{} }

func (v nonEmptyListValidator) Description(_ context.Context) string {
	return "must be omitted or contain at least one element (an explicit empty list is not allowed)"
}

func (v nonEmptyListValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v nonEmptyListValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if len(req.ConfigValue.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Empty list not allowed", nonEmptyListDetail)
	}
}

// nonEmptyMapValidator rejects a known, zero-length map.
type nonEmptyMapValidator struct{}

// nonEmptyMap returns a validator that rejects an explicit empty map.
func nonEmptyMap() validator.Map { return nonEmptyMapValidator{} }

func (v nonEmptyMapValidator) Description(_ context.Context) string {
	return "must be omitted or contain at least one entry (an explicit empty map is not allowed)"
}

func (v nonEmptyMapValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v nonEmptyMapValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if len(req.ConfigValue.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Empty map not allowed", nonEmptyMapDetail)
	}
}
