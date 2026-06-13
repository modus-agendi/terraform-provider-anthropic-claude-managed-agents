package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFiveFieldCron_ValidateString(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		config    types.String
		wantError bool
	}{
		{"valid 5-field", types.StringValue("0 3 * * *"), false},
		{"valid 5-field with ranges", types.StringValue("*/15 0-6 1,15 * 1-5"), false},
		{"@daily shortcut rejected", types.StringValue("@daily"), true},
		{"4 fields rejected", types.StringValue("0 3 * *"), true},
		{"6 fields rejected", types.StringValue("0 3 * * * *"), true},
		{"empty string rejected", types.StringValue(""), true},
		{"null -> ok", types.StringNull(), false},
		{"unknown -> ok", types.StringUnknown(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			fiveFieldCron().ValidateString(ctx, validator.StringRequest{
				Path:        path.Root("schedule").AtName("expression"),
				ConfigValue: tc.config,
			}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Fatalf("HasError = %v, want %v (diags: %v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
