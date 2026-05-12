package tools

import (
	"context"
	"fmt"
	"net/url"

	"ado-slim/internal/ado"
	"ado-slim/internal/slim"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func configureCoreTools(s *server.MCPServer, c *ado.Client) {
	// list_projects
	s.AddTool(mcp.NewTool("list_projects",
		mcp.WithDescription("List all projects in the Azure DevOps organization"),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		urlStr := c.OrgURL() + "/_apis/projects?api-version=7.1"
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimProject, 0, len(resp.Value))
		for _, p := range resp.Value {
			state := "unknown"
			if v, ok := p["state"]; ok && v != nil {
				state = fmt.Sprint(v)
			}
			items = append(items, slim.SlimProject{
				ID:             stringOf(p["id"]),
				Name:           stringOf(p["name"]),
				Description:    stringOf(p["description"]),
				State:          state,
				LastUpdateTime: slim.ToIsoDate(p["lastUpdateTime"]),
			})
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// list_project_teams
	s.AddTool(mcp.NewTool("list_project_teams",
		mcp.WithDescription("List teams within a project"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/_apis/projects/%s/teams?api-version=7.1",
			c.OrgURL(), url.PathEscape(project))
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimTeam, 0, len(resp.Value))
		for _, te := range resp.Value {
			items = append(items, slim.SlimTeam{
				ID:          stringOf(te["id"]),
				Name:        stringOf(te["name"]),
				Description: stringOf(te["description"]),
			})
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// get_identity_ids
	s.AddTool(mcp.NewTool("get_identity_ids",
		mcp.WithDescription("Look up Azure DevOps user identities by search filter (name or email)"),
		mcp.WithString("searchFilter", mcp.Required(), mcp.Description("Search term: user name or email")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filter, err := reqString(req, "searchFilter")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/_apis/identities?searchFilter=General&filterValue=%s&api-version=7.1",
			c.OrgURL(), url.QueryEscape(filter))
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimIdentityLookup, 0, len(resp.Value))
		for _, id := range resp.Value {
			email := propString(id, "Mail")
			if email == "" {
				email = propString(id, "Account")
			}
			displayName := stringOf(id["providerDisplayName"])
			if displayName == "" {
				displayName = stringOf(id["displayName"])
			}
			items = append(items, slim.SlimIdentityLookup{
				ID:          stringOf(id["id"]),
				DisplayName: displayName,
				Email:       email,
			})
		}
		// Note: TS quirk — no hasMore for this tool.
		return jsonResult(map[string]any{"count": len(items), "items": items})
	}))
}

// stringOf coerces interface values to a string, returning "" for nil.
func stringOf(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// propString reads ADO identity-properties shape: properties.<key>.$value
func propString(id map[string]any, key string) string {
	props, ok := id["properties"].(map[string]any)
	if !ok {
		return ""
	}
	entry, ok := props[key].(map[string]any)
	if !ok {
		return ""
	}
	if v, ok := entry["$value"].(string); ok {
		return v
	}
	return ""
}
