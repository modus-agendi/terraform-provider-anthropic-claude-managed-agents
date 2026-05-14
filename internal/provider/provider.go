// Package provider implements the terraform-plugin-framework Provider for
// Claude Managed Agents.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

// Ensure the implementation satisfies the expected interfaces.
var _ provider.Provider = (*claudeProvider)(nil)

type claudeProvider struct {
	version string
	commit  string
}

// New returns a function that constructs the provider. The function signature
// is the one providerserver.Serve expects.
func New(version, commit string) func() provider.Provider {
	return func() provider.Provider {
		return &claudeProvider{version: version, commit: commit}
	}
}

func (p *claudeProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "claude-managed-agents"
	resp.Version = p.version
}

func (p *claudeProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for Anthropic Claude Managed Agents (community, unofficial).",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Anthropic API key. Defaults to the `ANTHROPIC_API_KEY` environment variable. Marked sensitive: not shown in plan output.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "API base URL. Defaults to `https://api.anthropic.com`. Override for self-hosted gateways or to point at a local test server.",
				Optional:            true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of retries for transient failures (5xx, 429). Defaults to 3.",
				Optional:            true,
			},
		},
	}
}

func (p *claudeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := cfg.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Anthropic API key",
			"Set the `api_key` provider attribute or the `ANTHROPIC_API_KEY` environment variable.",
		)
		return
	}

	baseURL := cfg.BaseURL.ValueString()
	maxRetries := 0
	if !cfg.MaxRetries.IsNull() && !cfg.MaxRetries.IsUnknown() {
		maxRetries = int(cfg.MaxRetries.ValueInt64())
	}

	userAgent := "terraform-provider-claude-managed-agents/" + p.version
	c, err := client.New(client.Config{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		UserAgent:  userAgent,
		MaxRetries: maxRetries,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to construct API client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *claudeProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newAgentResource,
		newEnvironmentResource,
		newMemoryStoreResource,
		newSkillResource,
		newVaultResource,
		newVaultCredentialResource,
	}
}

func (p *claudeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newAgentDataSource,
		newAgentVersionDataSource,
		newEnvironmentDataSource,
		newFileDataSource,
		newMemoryStoreDataSource,
		newSkillDataSource,
		newVaultDataSource,
		newVaultCredentialDataSource,
	}
}
