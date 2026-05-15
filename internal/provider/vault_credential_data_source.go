package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-anthropic-claude-managed-agents/internal/client"
)

var (
	_ datasource.DataSource              = (*vaultCredentialDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*vaultCredentialDataSource)(nil)
)

type vaultCredentialDataSource struct {
	client *client.Client
}

func newVaultCredentialDataSource() datasource.DataSource {
	return &vaultCredentialDataSource{}
}

func (d *vaultCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_credential"
}

func (d *vaultCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing vault credential by `vault_id` and credential `id`. Secret payloads are never returned by the API and so are never populated by this data source.",
		Attributes: map[string]schema.Attribute{
			"vault_id":     schema.StringAttribute{Required: true, MarkdownDescription: "Parent vault id."},
			"id":           schema.StringAttribute{Required: true, MarkdownDescription: "Credential id (`cred_*`)."},
			"display_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Credential display name."},
			"auth": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Auth shape (secrets omitted).",
				Attributes: map[string]schema.Attribute{
					"type":           schema.StringAttribute{Computed: true, MarkdownDescription: "`static_bearer` or `mcp_oauth`."},
					"mcp_server_url": schema.StringAttribute{Computed: true, MarkdownDescription: "MCP server URL this credential is bound to."},
					"expires_at":     schema.StringAttribute{Computed: true, MarkdownDescription: "Access token expiry (mcp_oauth only)."},
					"refresh": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "OAuth refresh config (mcp_oauth only).",
						Attributes: map[string]schema.Attribute{
							"token_endpoint": schema.StringAttribute{Computed: true, MarkdownDescription: "OAuth token endpoint."},
							"client_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "OAuth client_id."},
							"scope":          schema.StringAttribute{Computed: true, MarkdownDescription: "OAuth scope."},
							"token_endpoint_auth": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Token endpoint auth.",
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{Computed: true, MarkdownDescription: "One of `none`, `client_secret_basic`, `client_secret_post`."},
								},
							},
						},
					},
				},
			},
			"created_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 creation timestamp."},
			"updated_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 last-modified timestamp."},
			"archived_at": schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 8601 archive timestamp, or null."},
		},
	}
}

func (d *vaultCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected ProviderData type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *vaultCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg vaultCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := d.client.GetVaultCredential(ctx, cfg.VaultID.ValueString(), cfg.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Vault credential not found", fmt.Sprintf("no credential %q in vault %q", cfg.ID.ValueString(), cfg.VaultID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read vault credential", err.Error())
		return
	}

	state := vaultCredentialDataSourceModel{
		ID:          types.StringValue(cred.ID),
		VaultID:     types.StringValue(cred.VaultID),
		DisplayName: types.StringValue(cred.DisplayName),
		CreatedAt:   types.StringValue(cred.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt:   types.StringValue(cred.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if cred.ArchivedAt != nil {
		state.ArchivedAt = types.StringValue(cred.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		state.ArchivedAt = types.StringNull()
	}

	expiresAt := types.StringNull()
	if cred.Auth.ExpiresAt != nil {
		expiresAt = types.StringValue(cred.Auth.ExpiresAt.Format(timeFormatRFC3339))
	}

	refreshObj := types.ObjectNull(credentialDSRefreshAttrTypes())
	if cred.Auth.Refresh != nil {
		eaObj, eaDiags := types.ObjectValue(credentialDSEndpointAuthAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(cred.Auth.Refresh.TokenEndpointAuth.Type),
		})
		resp.Diagnostics.Append(eaDiags...)
		scope := types.StringNull()
		if cred.Auth.Refresh.Scope != "" {
			scope = types.StringValue(cred.Auth.Refresh.Scope)
		}
		rObj, rDiags := types.ObjectValue(credentialDSRefreshAttrTypes(), map[string]attr.Value{
			"token_endpoint":      types.StringValue(cred.Auth.Refresh.TokenEndpoint),
			"client_id":           types.StringValue(cred.Auth.Refresh.ClientID),
			"scope":               scope,
			"token_endpoint_auth": eaObj,
		})
		resp.Diagnostics.Append(rDiags...)
		refreshObj = rObj
	}

	authObj, aDiags := types.ObjectValue(credentialDSAuthAttrTypes(), map[string]attr.Value{
		"type":           types.StringValue(cred.Auth.Type),
		"mcp_server_url": types.StringValue(cred.Auth.McpServerURL),
		"expires_at":     expiresAt,
		"refresh":        refreshObj,
	})
	resp.Diagnostics.Append(aDiags...)
	state.Auth = authObj

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type vaultCredentialDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	VaultID     types.String `tfsdk:"vault_id"`
	DisplayName types.String `tfsdk:"display_name"`
	Auth        types.Object `tfsdk:"auth"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ArchivedAt  types.String `tfsdk:"archived_at"`
}

func credentialDSAuthAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":           types.StringType,
		"mcp_server_url": types.StringType,
		"expires_at":     types.StringType,
		"refresh":        types.ObjectType{AttrTypes: credentialDSRefreshAttrTypes()},
	}
}

func credentialDSRefreshAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"token_endpoint":      types.StringType,
		"client_id":           types.StringType,
		"scope":               types.StringType,
		"token_endpoint_auth": types.ObjectType{AttrTypes: credentialDSEndpointAuthAttrTypes()},
	}
}

func credentialDSEndpointAuthAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
	}
}
