package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// fiveFieldCronValidator rejects a cron expression that is not exactly five
// whitespace-separated fields. The deployments API accepts only 5-field POSIX
// cron (e.g. "0 3 * * *") and rejects "@daily"-style shortcuts; catching the
// common mistakes at plan time gives a clear error instead of an opaque API
// rejection at apply time.
type fiveFieldCronValidator struct{}

// fiveFieldCron returns a validator that requires a 5-field POSIX cron string.
func fiveFieldCron() validator.String { return fiveFieldCronValidator{} }

func (v fiveFieldCronValidator) Description(_ context.Context) string {
	return "must be a 5-field POSIX cron expression (e.g. \"0 3 * * *\"); shortcuts like @daily are not supported"
}

func (v fiveFieldCronValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v fiveFieldCronValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	expr := req.ConfigValue.ValueString()
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid cron expression",
			fmt.Sprintf("schedule.expression must be a 5-field POSIX cron expression (e.g. \"0 3 * * *\"); got %d field(s) in %q. Shortcuts like @daily are not supported.", len(fields), expr),
		)
	}
}
