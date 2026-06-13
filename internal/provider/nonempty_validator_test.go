package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNonEmptyList_ValidateList(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		config    types.List
		wantError bool
	}{
		{"known empty -> error", types.ListValueMust(types.StringType, []attr.Value{}), true},
		{"non-empty -> ok", types.ListValueMust(types.StringType, []attr.Value{types.StringValue("x")}), false},
		{"null -> ok", types.ListNull(types.StringType), false},
		{"unknown -> ok", types.ListUnknown(types.StringType), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.ListResponse{}
			nonEmptyList().ValidateList(ctx, validator.ListRequest{Path: path.Root("allowed_hosts"), ConfigValue: tc.config}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Fatalf("HasError = %v, want %v (diags: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestNonEmptyMap_ValidateMap(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		config    types.Map
		wantError bool
	}{
		{"known empty -> error", types.MapValueMust(types.StringType, map[string]attr.Value{}), true},
		{"non-empty -> ok", types.MapValueMust(types.StringType, map[string]attr.Value{"k": types.StringValue("v")}), false},
		{"null -> ok", types.MapNull(types.StringType), false},
		{"unknown -> ok", types.MapUnknown(types.StringType), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.MapResponse{}
			nonEmptyMap().ValidateMap(ctx, validator.MapRequest{Path: path.Root("metadata"), ConfigValue: tc.config}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Fatalf("HasError = %v, want %v (diags: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
