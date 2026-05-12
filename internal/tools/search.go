package tools

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"ado-slim/internal/ado"
	"ado-slim/internal/slim"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func searchURL(c *ado.Client, area string) string {
	return c.SearchOrgURL() + "/_apis/search/" + area + "?api-version=7.1"
}

var (
	reSysPrefix = regexp.MustCompile(`^System\.`)
	reMSPrefix  = regexp.MustCompile(`^Microsoft\.VSTS\.\w+\.`)
)

func friendlyShort(name string) string {
	s := reSysPrefix.ReplaceAllString(name, "")
	s = reMSPrefix.ReplaceAllString(s, "")
	return s
}

func configureSearchTools(s *server.MCPServer, c *ado.Client) {
	// search_code
	s.AddTool(mcp.NewTool("search_code",
		mcp.WithDescription("Search code across repositories"),
		mcp.WithString("searchText", mcp.Required(), mcp.Description("Search text/pattern")),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name to scope search")),
		mcp.WithNumber("top", mcp.Description("Max results (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Results to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		searchText, err := reqString(req, "searchText")
		if err != nil {
			return nil, err
		}
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		body := map[string]any{
			"searchText": searchText,
			"$top":       t,
			"$skip":      sk,
			"filters":    map[string]any{"Project": []string{project}},
		}
		var resp map[string]any
		if err := c.PostJSON(ctx, searchURL(c, "codesearchresults"), body, &resp); err != nil {
			return nil, err
		}
		results, _ := resp["results"].([]any)
		items := make([]slim.SlimCodeSearchResult, 0, len(results))
		for _, r := range results {
			rm, _ := r.(map[string]any)
			matches := []slim.SlimSearchMatch{}
			if mr, ok := rm["matches"].(map[string]any); ok {
				for field, hitsRaw := range mr {
					hits, ok := hitsRaw.([]any)
					if !ok {
						continue
					}
					snippets := []string{}
					for _, hit := range hits {
						hm, _ := hit.(map[string]any)
						if sn := stringOf(hm["snippet"]); sn != "" {
							snippets = append(snippets, sn)
						}
					}
					if len(snippets) > 0 {
						matches = append(matches, slim.SlimSearchMatch{Field: field, Snippets: snippets})
					}
				}
			}
			repoName := ""
			if rep, ok := rm["repository"].(map[string]any); ok {
				repoName = stringOf(rep["name"])
			}
			projName := project
			if p, ok := rm["project"].(map[string]any); ok {
				if n := stringOf(p["name"]); n != "" {
					projName = n
				}
			}
			branch := ""
			if vs, ok := rm["versions"].([]any); ok && len(vs) > 0 {
				if vm, ok := vs[0].(map[string]any); ok {
					branch = slim.StripRefPrefix(stringOf(vm["branchName"]))
				}
			}
			items = append(items, slim.SlimCodeSearchResult{
				FileName: stringOf(rm["fileName"]),
				Path:     stringOf(rm["path"]),
				Repo:     repoName,
				Project:  projName,
				Branch:   branch,
				Matches:  matches,
			})
		}
		count := intFromAny(resp["count"])
		return jsonResult(map[string]any{
			"count":   len(items),
			"hasMore": count > sk+t,
			"items":   items,
		})
	}))

	// search_wiki
	s.AddTool(mcp.NewTool("search_wiki",
		mcp.WithDescription("Search wiki pages across the organization"),
		mcp.WithString("searchText", mcp.Required(), mcp.Description("Search text")),
		mcp.WithString("project", mcp.Description("Project name to scope search")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		searchText, err := reqString(req, "searchText")
		if err != nil {
			return nil, err
		}
		project := optString(req, "project")
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		body := map[string]any{
			"searchText": searchText,
			"$top":       t,
			"$skip":      sk,
		}
		if project != "" {
			body["filters"] = map[string]any{"Project": []string{project}}
		}
		var resp map[string]any
		if err := c.PostJSON(ctx, searchURL(c, "wikisearchresults"), body, &resp); err != nil {
			return nil, err
		}
		results, _ := resp["results"].([]any)
		items := make([]slim.SlimWikiSearchResult, 0, len(results))
		for _, r := range results {
			rm, _ := r.(map[string]any)
			snippet := ""
			if hits, ok := rm["hits"].([]any); ok {
				for _, h := range hits {
					hm, _ := h.(map[string]any)
					if hl, ok := hm["highlights"].([]any); ok && len(hl) > 0 {
						parts := make([]string, 0, len(hl))
						for _, p := range hl {
							parts = append(parts, stringOf(p))
						}
						snippet = strings.Join(parts, " ... ")
						break
					}
				}
			}
			fileName := stringOf(rm["fileName"])
			wikiName := ""
			if w, ok := rm["wiki"].(map[string]any); ok {
				wikiName = stringOf(w["name"])
			}
			title := fileName
			if wikiName != "" {
				title = wikiName + ": " + fileName
			}
			projName := project
			if p, ok := rm["project"].(map[string]any); ok {
				if n := stringOf(p["name"]); n != "" {
					projName = n
				}
			}
			if snippet == "" {
				snippet = fileName
			}
			items = append(items, slim.SlimWikiSearchResult{
				Title:    title,
				Path:     stringOf(rm["path"]),
				Project:  projName,
				WikiName: wikiName,
				Snippet:  snippet,
			})
		}
		count := intFromAny(resp["count"])
		return jsonResult(map[string]any{
			"count":   len(items),
			"hasMore": count > sk+t,
			"items":   items,
		})
	}))

	// search_workitem
	s.AddTool(mcp.NewTool("search_workitem",
		mcp.WithDescription("Search work items across the organization"),
		mcp.WithString("searchText", mcp.Required(), mcp.Description("Search text")),
		mcp.WithString("project", mcp.Description("Project name to scope search")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		searchText, err := reqString(req, "searchText")
		if err != nil {
			return nil, err
		}
		project := optString(req, "project")
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		body := map[string]any{
			"searchText": searchText,
			"$top":       t,
			"$skip":      sk,
		}
		if project != "" {
			body["filters"] = map[string]any{"System.TeamProject": []string{project}}
		}
		var resp map[string]any
		if err := c.PostJSON(ctx, searchURL(c, "workitemsearchresults"), body, &resp); err != nil {
			return nil, err
		}
		results, _ := resp["results"].([]any)
		items := make([]slim.SlimWorkItemSearchResult, 0, len(results))
		for _, r := range results {
			rm, _ := r.(map[string]any)
			fields, _ := rm["fields"].(map[string]any)
			matched := []string{}
			snippet := ""
			if hits, ok := rm["hits"].([]any); ok {
				for _, h := range hits {
					hm, _ := h.(map[string]any)
					if ref := stringOf(hm["fieldReferenceName"]); ref != "" {
						matched = append(matched, friendlyShort(ref))
					}
					if snippet == "" {
						if hl, ok := hm["highlights"].([]any); ok && len(hl) > 0 {
							parts := make([]string, 0, len(hl))
							for _, p := range hl {
								parts = append(parts, stringOf(p))
							}
							snippet = strings.Join(parts, " ... ")
						}
					}
				}
			}
			id := 0
			if v, ok := fields["system.id"]; ok && v != nil {
				switch n := v.(type) {
				case float64:
					id = int(n)
				case string:
					id, _ = strconv.Atoi(n)
				}
			}
			projName := project
			if p, ok := rm["project"].(map[string]any); ok {
				if n := stringOf(p["name"]); n != "" {
					projName = n
				}
			}
			out := slim.SlimWorkItemSearchResult{
				ID:            id,
				Type:          stringOf(fields["system.workitemtype"]),
				Title:         stringOf(fields["system.title"]),
				State:         stringOf(fields["system.state"]),
				Project:       projName,
				Snippet:       snippet,
				MatchedFields: matched,
			}
			if v, ok := fields["system.assignedto"]; ok && v != nil {
				out.AssignedTo = slim.FlattenIdentity(v)
			}
			items = append(items, out)
		}
		count := intFromAny(resp["count"])
		return jsonResult(map[string]any{
			"count":   len(items),
			"hasMore": count > sk+t,
			"items":   items,
		})
	}))
}
