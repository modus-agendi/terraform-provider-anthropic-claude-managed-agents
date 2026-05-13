package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/andasv/terraform-provider-claude-managed-agents/internal/client"
)

var (
	_ resource.Resource                = (*vaultCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*vaultCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*vaultCredentialResource)(nil)
)

type vaultCredentialResource struct {
	client *client.Client
}

func newVaultCredentialResource() resource.Resource {
	return &vaultCredentialResource{}
}

func (r *vaultCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_credential"
}

func (r *vaultCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: vaultCredentialResourceMarkdown,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned identifier (e.g. `cred_01ABC...`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vault_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the parent vault. Immutable; changing forces replacement.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable credential name. Mutable.",
			},
			"auth": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Auth payload. Discriminated on `auth.type`: `static_bearer` carries a single bearer token; `mcp_oauth` carries an access token + optional refresh block.",
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Either `static_bearer` or `mcp_oauth`. Immutable; changing forces replacement.",
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"mcp_server_url": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "MCP server URL this credential is bound to. Immutable; changing forces replacement. The API rejects duplicate URLs within the same vault.",
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"token": schema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						WriteOnly:           true,
						MarkdownDescription: "Bearer token for `static_bearer` auth. Write-only: never persisted to state. Pair with `token_wo_version` to trigger rotation.",
					},
					"token_wo_version": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Increment this integer to signal that the corresponding write-only secret has been rotated and should be re-sent to the API.",
					},
					"access_token": schema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						WriteOnly:           true,
						MarkdownDescription: "Access token for `mcp_oauth` auth. Write-only. Pair with `access_token_wo_version` for rotation.",
					},
					"access_token_wo_version": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Rotation counter for `access_token`.",
					},
					"expires_at": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "RFC 3339 timestamp at which the access token expires. Only meaningful for `mcp_oauth`.",
					},
					"refresh": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "OAuth refresh configuration. Set when you want Anthropic to refresh the access token on your behalf.",
						Attributes: map[string]schema.Attribute{
							"token_endpoint": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "OAuth token endpoint URL. Immutable; changing forces replacement.",
								PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
							},
							"client_id": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "OAuth client_id. Immutable; changing forces replacement.",
								PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
							},
							"scope": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Space-delimited OAuth scopes. Mutable.",
							},
							"refresh_token": schema.StringAttribute{
								Optional:            true,
								Sensitive:           true,
								WriteOnly:           true,
								MarkdownDescription: "OAuth refresh token. Write-only. Pair with `refresh_token_wo_version` for rotation.",
							},
							"refresh_token_wo_version": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "Rotation counter for `refresh_token`.",
							},
							"token_endpoint_auth": schema.SingleNestedAttribute{
								Required:            true,
								MarkdownDescription: "How to authenticate the refresh call. Type `none` for public clients, `client_secret_basic` for HTTP Basic, `client_secret_post` for body-form.",
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{Required: true, MarkdownDescription: "One of `none`, `client_secret_basic`, `client_secret_post`."},
									"client_secret": schema.StringAttribute{
										Optional:            true,
										Sensitive:           true,
										WriteOnly:           true,
										MarkdownDescription: "Client secret for non-`none` token-endpoint-auth types. Write-only. Pair with `client_secret_wo_version` for rotation.",
									},
									"client_secret_wo_version": schema.Int64Attribute{
										Optional:            true,
										MarkdownDescription: "Rotation counter for `client_secret`.",
									},
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

func (r *vaultCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected ProviderData type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *vaultCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config vaultCredentialModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authMap, diags := credentialAuthToAPI(ctx, plan.Auth, config.Auth)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := r.client.CreateVaultCredential(ctx, plan.VaultID.ValueString(), client.VaultCredentialCreateRequest{
		DisplayName: plan.DisplayName.ValueString(),
		Auth:        authMap,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create vault credential", err.Error())
		return
	}

	state, sDiags := vaultCredentialFromAPI(ctx, cred, plan.Auth)
	resp.Diagnostics.Append(sDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *vaultCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := r.client.GetVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read vault credential", err.Error())
		return
	}

	fresh, diags := vaultCredentialFromAPI(ctx, cred, state.Auth)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *vaultCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config vaultCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := client.VaultCredentialUpdateRequest{}
	if !plan.DisplayName.Equal(state.DisplayName) {
		v := plan.DisplayName.ValueString()
		updateReq.DisplayName = &v
	}

	authMap, diags := credentialAuthUpdatePayload(ctx, plan.Auth, state.Auth, config.Auth)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(authMap) > 0 {
		updateReq.Auth = authMap
	}

	cred, err := r.client.UpdateVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update vault credential", err.Error())
		return
	}

	fresh, sDiags := vaultCredentialFromAPI(ctx, cred, plan.Auth)
	resp.Diagnostics.Append(sDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, fresh)...)
}

func (r *vaultCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.ArchiveVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to archive vault credential", err.Error())
	}
}

// ImportState accepts the composite "vault_id:credential_id" identifier.
func (r *vaultCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := splitImportID(req.ID, 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import id", "expected `vault_id:credential_id`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vault_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

type vaultCredentialModel struct {
	ID          types.String `tfsdk:"id"`
	VaultID     types.String `tfsdk:"vault_id"`
	DisplayName types.String `tfsdk:"display_name"`
	Auth        types.Object `tfsdk:"auth"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	ArchivedAt  types.String `tfsdk:"archived_at"`
}

func credentialAuthAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":                    types.StringType,
		"mcp_server_url":          types.StringType,
		"token":                   types.StringType,
		"token_wo_version":        types.Int64Type,
		"access_token":            types.StringType,
		"access_token_wo_version": types.Int64Type,
		"expires_at":              types.StringType,
		"refresh":                 types.ObjectType{AttrTypes: credentialRefreshAttrTypes()},
	}
}

func credentialRefreshAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"token_endpoint":           types.StringType,
		"client_id":                types.StringType,
		"scope":                    types.StringType,
		"refresh_token":            types.StringType,
		"refresh_token_wo_version": types.Int64Type,
		"token_endpoint_auth":      types.ObjectType{AttrTypes: credentialEndpointAuthAttrTypes()},
	}
}

func credentialEndpointAuthAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":                     types.StringType,
		"client_secret":            types.StringType,
		"client_secret_wo_version": types.Int64Type,
	}
}

// splitImportID splits "a:b" into ["a","b"]. Returns an empty slice if the
// number of colon-separated parts does not match `n`.
func splitImportID(id string, n int) []string {
	parts := []string{}
	cur := ""
	for _, ch := range id {
		if ch == ':' {
			parts = append(parts, cur)
			cur = ""
			continue
		}
		cur += string(ch)
	}
	parts = append(parts, cur)
	if len(parts) != n {
		return nil
	}
	return parts
}

const vaultCredentialResourceMarkdown = "Manages a single credential within a vault. Bind a token or OAuth client to an MCP server URL so that future sessions referencing the parent vault can authenticate against that server.\n\n" +
	"### Secrets are write-only\n\n" +
	"`token`, `access_token`, `refresh_token`, and `client_secret` are TF 1.11 write-only attributes — they are sent to the API but never stored in state. To rotate a secret, increment the matching `*_wo_version` field; the provider re-sends the secret from your config on the next plan.\n\n" +
	"### Immutability\n\n" +
	"`auth.type`, `auth.mcp_server_url`, `auth.refresh.token_endpoint`, and `auth.refresh.client_id` are immutable. Changing any of them forces Terraform to destroy and re-create the credential. The API rejects creating two active credentials with the same `mcp_server_url` in the same vault.\n\n" +
	"### Lifecycle on destroy\n\n" +
	"`terraform destroy` archives the credential (`POST /archive`), which purges the secret payload while keeping the audit record visible. Use the parent `claude-managed-agents_vault` with `delete_on_destroy = true` to hard-delete a vault and its credentials together."

// credentialAuthToAPI builds the create payload from the plan (non-secret)
// and config (secret WriteOnly values).
func credentialAuthToAPI(ctx context.Context, planAuth types.Object, configAuth types.Object) (map[string]any, diag.Diagnostics) {
	return credentialAuthBuild(ctx, planAuth, configAuth, true)
}

// credentialAuthUpdatePayload builds the update payload, only including
// fields that actually changed between plan and state.
func credentialAuthUpdatePayload(ctx context.Context, planAuth, stateAuth, configAuth types.Object) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	planVals, d := credentialAuthDecode(ctx, planAuth)
	diags.Append(d...)
	stateVals, d := credentialAuthDecode(ctx, stateAuth)
	diags.Append(d...)
	configVals, d := credentialAuthDecode(ctx, configAuth)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{}
	out["type"] = planVals.typ // always include type so the API can route

	// Secret rotation: include the secret in the update payload whenever
	// the corresponding _wo_version changed.
	if !planVals.tokenWoVersion.Equal(stateVals.tokenWoVersion) {
		out["token"] = configVals.tokenSecret
	}
	if !planVals.accessTokenWoVersion.Equal(stateVals.accessTokenWoVersion) {
		out["access_token"] = configVals.accessTokenSecret
	}
	if planVals.expiresAt != stateVals.expiresAt && planVals.expiresAt != "" {
		out["expires_at"] = planVals.expiresAt
	}

	if !planVals.refresh.IsNull() {
		refresh := map[string]any{}
		if !planVals.refreshTokenWoVersion.Equal(stateVals.refreshTokenWoVersion) {
			refresh["refresh_token"] = configVals.refreshTokenSecret
		}
		if planVals.refreshScope != stateVals.refreshScope && planVals.refreshScope != "" {
			refresh["scope"] = planVals.refreshScope
		}
		if !planVals.clientSecretWoVersion.Equal(stateVals.clientSecretWoVersion) {
			refresh["token_endpoint_auth"] = map[string]any{
				"type":          planVals.endpointAuthType,
				"client_secret": configVals.clientSecret,
			}
		}
		if len(refresh) > 0 {
			out["refresh"] = refresh
		}
	}

	if len(out) == 1 { // only contains the type discriminator
		return nil, diags
	}
	return out, diags
}

func credentialAuthBuild(ctx context.Context, planAuth, configAuth types.Object, _ bool) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	planVals, d := credentialAuthDecode(ctx, planAuth)
	diags.Append(d...)
	configVals, d := credentialAuthDecode(ctx, configAuth)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	out := map[string]any{
		"type":           planVals.typ,
		"mcp_server_url": planVals.mcpServerURL,
	}
	switch planVals.typ {
	case "static_bearer":
		if configVals.tokenSecret != "" {
			out["token"] = configVals.tokenSecret
		}
	case "mcp_oauth":
		if configVals.accessTokenSecret != "" {
			out["access_token"] = configVals.accessTokenSecret
		}
		if planVals.expiresAt != "" {
			out["expires_at"] = planVals.expiresAt
		}
		if !planVals.refresh.IsNull() {
			refresh := map[string]any{
				"token_endpoint": planVals.refreshTokenEndpoint,
				"client_id":      planVals.refreshClientID,
			}
			if planVals.refreshScope != "" {
				refresh["scope"] = planVals.refreshScope
			}
			if configVals.refreshTokenSecret != "" {
				refresh["refresh_token"] = configVals.refreshTokenSecret
			}
			endpointAuth := map[string]any{"type": planVals.endpointAuthType}
			if configVals.clientSecret != "" {
				endpointAuth["client_secret"] = configVals.clientSecret
			}
			refresh["token_endpoint_auth"] = endpointAuth
			out["refresh"] = refresh
		}
	}
	return out, diags
}

// authDecoded is the de-tfsdk'd form of the auth nested object. Splitting
// into a struct keeps the create/update builders readable.
type authDecoded struct {
	typ                   string
	mcpServerURL          string
	tokenSecret           string
	tokenWoVersion        types.Int64
	accessTokenSecret     string
	accessTokenWoVersion  types.Int64
	expiresAt             string
	refresh               types.Object
	refreshTokenEndpoint  string
	refreshClientID       string
	refreshScope          string
	refreshTokenSecret    string
	refreshTokenWoVersion types.Int64
	endpointAuthType      string
	clientSecret          string
	clientSecretWoVersion types.Int64
}

func credentialAuthDecode(ctx context.Context, obj types.Object) (authDecoded, diag.Diagnostics) {
	var diags diag.Diagnostics
	var out authDecoded
	if obj.IsNull() || obj.IsUnknown() {
		return out, diags
	}

	var raw struct {
		Type                 types.String `tfsdk:"type"`
		McpServerURL         types.String `tfsdk:"mcp_server_url"`
		Token                types.String `tfsdk:"token"`
		TokenWoVersion       types.Int64  `tfsdk:"token_wo_version"`
		AccessToken          types.String `tfsdk:"access_token"`
		AccessTokenWoVersion types.Int64  `tfsdk:"access_token_wo_version"`
		ExpiresAt            types.String `tfsdk:"expires_at"`
		Refresh              types.Object `tfsdk:"refresh"`
	}
	diags.Append(obj.As(ctx, &raw, basicObjectAsOpts())...)
	if diags.HasError() {
		return out, diags
	}

	out.typ = raw.Type.ValueString()
	out.mcpServerURL = raw.McpServerURL.ValueString()
	out.tokenSecret = raw.Token.ValueString()
	out.tokenWoVersion = raw.TokenWoVersion
	out.accessTokenSecret = raw.AccessToken.ValueString()
	out.accessTokenWoVersion = raw.AccessTokenWoVersion
	out.expiresAt = raw.ExpiresAt.ValueString()
	out.refresh = raw.Refresh

	if !raw.Refresh.IsNull() && !raw.Refresh.IsUnknown() {
		var refresh struct {
			TokenEndpoint         types.String `tfsdk:"token_endpoint"`
			ClientID              types.String `tfsdk:"client_id"`
			Scope                 types.String `tfsdk:"scope"`
			RefreshToken          types.String `tfsdk:"refresh_token"`
			RefreshTokenWoVersion types.Int64  `tfsdk:"refresh_token_wo_version"`
			TokenEndpointAuth     types.Object `tfsdk:"token_endpoint_auth"`
		}
		diags.Append(raw.Refresh.As(ctx, &refresh, basicObjectAsOpts())...)
		if diags.HasError() {
			return out, diags
		}
		out.refreshTokenEndpoint = refresh.TokenEndpoint.ValueString()
		out.refreshClientID = refresh.ClientID.ValueString()
		out.refreshScope = refresh.Scope.ValueString()
		out.refreshTokenSecret = refresh.RefreshToken.ValueString()
		out.refreshTokenWoVersion = refresh.RefreshTokenWoVersion

		if !refresh.TokenEndpointAuth.IsNull() && !refresh.TokenEndpointAuth.IsUnknown() {
			var ea struct {
				Type                  types.String `tfsdk:"type"`
				ClientSecret          types.String `tfsdk:"client_secret"`
				ClientSecretWoVersion types.Int64  `tfsdk:"client_secret_wo_version"`
			}
			diags.Append(refresh.TokenEndpointAuth.As(ctx, &ea, basicObjectAsOpts())...)
			out.endpointAuthType = ea.Type.ValueString()
			out.clientSecret = ea.ClientSecret.ValueString()
			out.clientSecretWoVersion = ea.ClientSecretWoVersion
		}
	}
	return out, diags
}

// vaultCredentialFromAPI maps the API response onto state, preserving the
// write-only secret values and rotation counters from the plan/state object
// that already had them. The API never returns secrets; we must keep the
// non-state representation locally.
func vaultCredentialFromAPI(ctx context.Context, c *client.VaultCredential, priorAuth types.Object) (vaultCredentialModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	m := vaultCredentialModel{
		ID:          types.StringValue(c.ID),
		VaultID:     types.StringValue(c.VaultID),
		DisplayName: types.StringValue(c.DisplayName),
		CreatedAt:   types.StringValue(c.CreatedAt.Format(timeFormatRFC3339)),
		UpdatedAt:   types.StringValue(c.UpdatedAt.Format(timeFormatRFC3339)),
	}
	if c.ArchivedAt != nil {
		m.ArchivedAt = types.StringValue(c.ArchivedAt.Format(timeFormatRFC3339))
	} else {
		m.ArchivedAt = types.StringNull()
	}

	// Read prior _wo_version + non-secret fields from priorAuth to preserve
	// the rotation counters across refresh.
	prior, d := credentialAuthDecode(ctx, priorAuth)
	diags.Append(d...)
	if diags.HasError() {
		return m, diags
	}

	expiresAt := types.StringNull()
	if c.Auth.ExpiresAt != nil {
		expiresAt = types.StringValue(c.Auth.ExpiresAt.Format(timeFormatRFC3339))
	} else if prior.expiresAt != "" {
		expiresAt = types.StringValue(prior.expiresAt)
	}

	refreshObj := types.ObjectNull(credentialRefreshAttrTypes())
	if c.Auth.Refresh != nil {
		eaObj, eaDiags := types.ObjectValue(credentialEndpointAuthAttrTypes(), map[string]attr.Value{
			"type":                     types.StringValue(c.Auth.Refresh.TokenEndpointAuth.Type),
			"client_secret":            types.StringNull(),
			"client_secret_wo_version": prior.clientSecretWoVersion,
		})
		diags.Append(eaDiags...)

		scope := types.StringNull()
		if c.Auth.Refresh.Scope != "" {
			scope = types.StringValue(c.Auth.Refresh.Scope)
		}
		obj, rDiags := types.ObjectValue(credentialRefreshAttrTypes(), map[string]attr.Value{
			"token_endpoint":           types.StringValue(c.Auth.Refresh.TokenEndpoint),
			"client_id":                types.StringValue(c.Auth.Refresh.ClientID),
			"scope":                    scope,
			"refresh_token":            types.StringNull(),
			"refresh_token_wo_version": prior.refreshTokenWoVersion,
			"token_endpoint_auth":      eaObj,
		})
		diags.Append(rDiags...)
		refreshObj = obj
	}

	authObj, aDiags := types.ObjectValue(credentialAuthAttrTypes(), map[string]attr.Value{
		"type":                    types.StringValue(c.Auth.Type),
		"mcp_server_url":          types.StringValue(c.Auth.McpServerURL),
		"token":                   types.StringNull(),
		"token_wo_version":        prior.tokenWoVersion,
		"access_token":            types.StringNull(),
		"access_token_wo_version": prior.accessTokenWoVersion,
		"expires_at":              expiresAt,
		"refresh":                 refreshObj,
	})
	diags.Append(aDiags...)
	m.Auth = authObj

	return m, diags
}
