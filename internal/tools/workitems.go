package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"ado-slim/internal/ado"
	"ado-slim/internal/slim"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// === field-extraction helpers ===

func fieldFloatPtr(fields map[string]any, key string) *float64 {
	v, ok := fields[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		x := n
		return &x
	case int:
		x := float64(n)
		return &x
	case int64:
		x := float64(n)
		return &x
	}
	// strings unparseable here, leave nil
	return nil
}

func fieldString(fields map[string]any, key string) string {
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return slim.FlattenIdentity(t)
	}
	return fmt.Sprint(v)
}

// stringFieldRaw returns the value as printed by JS String(): identity objects
// flatten, primitives stringify, missing returns "".
func stringFieldRaw(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return slim.FlattenIdentity(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		// JS String(num) — drop trailing .0 for integral floats
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

func transformWorkItem(orgURL string, wi map[string]any, extraFields []string) slim.SlimWorkItem {
	fields, _ := wi["fields"].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}

	relations := []slim.SlimRelation{}
	if rels, ok := wi["relations"].([]any); ok {
		for _, r := range rels {
			rm, _ := r.(map[string]any)
			urlStr := stringOf(rm["url"])
			if urlStr == "" {
				continue
			}
			id := slim.ExtractWorkItemIDFromURL(urlStr)
			if id < 0 {
				continue
			}
			relations = append(relations, slim.SlimRelation{
				Type:     slim.FriendlyRelationType(stringOf(rm["rel"])),
				TargetID: id,
			})
		}
	}

	project := stringOf(fields["System.TeamProject"])
	id := 0
	if v, ok := wi["id"].(float64); ok {
		id = int(v)
	}

	out := slim.SlimWorkItem{
		ID:                 id,
		Type:               stringOf(fields["System.WorkItemType"]),
		Title:              stringOf(fields["System.Title"]),
		State:              stringOf(fields["System.State"]),
		Reason:             stringOf(fields["System.Reason"]),
		CreatedBy:          slim.FlattenIdentity(fields["System.CreatedBy"]),
		CreatedDate:        slim.ToIsoDate(fields["System.CreatedDate"]),
		ChangedDate:        slim.ToIsoDate(fields["System.ChangedDate"]),
		IterationPath:      stringOf(fields["System.IterationPath"]),
		AreaPath:           stringOf(fields["System.AreaPath"]),
		Tags:               stringOf(fields["System.Tags"]),
		Relations:          relations,
		Description:        slim.StripHtml(stringOf(fields["System.Description"])),
		AcceptanceCriteria: slim.StripHtml(stringOf(fields["Microsoft.VSTS.Common.AcceptanceCriteria"])),
		Priority:           fieldFloatPtr(fields, "Microsoft.VSTS.Common.Priority"),
		StoryPoints:        fieldFloatPtr(fields, "Microsoft.VSTS.Scheduling.StoryPoints"),
		RemainingWork:      fieldFloatPtr(fields, "Microsoft.VSTS.Scheduling.RemainingWork"),
		OriginalEstimate:   fieldFloatPtr(fields, "Microsoft.VSTS.Scheduling.OriginalEstimate"),
		CompletedWork:      fieldFloatPtr(fields, "Microsoft.VSTS.Scheduling.CompletedWork"),
		WebURL:             fmt.Sprintf("%s/%s/_workitems/edit/%d", orgURL, url.PathEscape(project), id),
	}

	if v, ok := fields["System.AssignedTo"]; ok && v != nil {
		out.AssignedTo = slim.FlattenIdentity(v)
	}
	if v, ok := fields["Microsoft.VSTS.Common.ResolvedBy"]; ok && v != nil {
		out.ResolvedBy = slim.FlattenIdentity(v)
	}
	if v, ok := fields["Microsoft.VSTS.Common.ResolvedDate"]; ok && v != nil {
		out.ResolvedDate = slim.ToIsoDate(v)
	}
	if v, ok := fields["Microsoft.VSTS.Common.ClosedBy"]; ok && v != nil {
		out.ClosedBy = slim.FlattenIdentity(v)
	}
	if v, ok := fields["Microsoft.VSTS.Common.ClosedDate"]; ok && v != nil {
		out.ClosedDate = slim.ToIsoDate(v)
	}
	if v, ok := fields["Microsoft.VSTS.Common.Severity"]; ok && v != nil {
		out.Severity = fmt.Sprint(v)
	}

	if extra := buildExtraFields(fields, extraFields); extra != nil {
		out.ExtraFields = extra
	}
	return out
}

func buildExtraFields(fields map[string]any, refs []string) map[string]string {
	if len(refs) == 0 {
		return nil
	}
	bag := map[string]string{}
	for _, ref := range refs {
		v, ok := fields[ref]
		if !ok || v == nil {
			continue
		}
		key := slim.FriendlyFieldName(ref)
		switch t := v.(type) {
		case map[string]any:
			bag[key] = slim.FlattenIdentity(t)
		case string:
			text := slim.StripHtml(t)
			if text != "" {
				bag[key] = text
			}
		default:
			bag[key] = fmt.Sprint(t)
		}
	}
	if len(bag) == 0 {
		return nil
	}
	return bag
}

func transformWorkItemSummary(wi map[string]any) slim.SlimWorkItemSummary {
	fields, _ := wi["fields"].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}
	id := 0
	if v, ok := wi["id"].(float64); ok {
		id = int(v)
	}
	out := slim.SlimWorkItemSummary{
		ID:            id,
		Type:          stringOf(fields["System.WorkItemType"]),
		Title:         stringOf(fields["System.Title"]),
		State:         stringOf(fields["System.State"]),
		Priority:      fieldFloatPtr(fields, "Microsoft.VSTS.Common.Priority"),
		StoryPoints:   fieldFloatPtr(fields, "Microsoft.VSTS.Scheduling.StoryPoints"),
		IterationPath: stringOf(fields["System.IterationPath"]),
		AreaPath:      stringOf(fields["System.AreaPath"]),
		Tags:          stringOf(fields["System.Tags"]),
	}
	if v, ok := fields["System.AssignedTo"]; ok && v != nil {
		out.AssignedTo = slim.FlattenIdentity(v)
	}
	return out
}

// fetchWorkItemsByIDs batches an arbitrarily large id list into 200-id chunks
// and pulls summary-shaped work items.
func fetchWorkItemsByIDs(ctx context.Context, c *ado.Client, ids []int) ([]slim.SlimWorkItemSummary, error) {
	if len(ids) == 0 {
		return []slim.SlimWorkItemSummary{}, nil
	}
	out := make([]slim.SlimWorkItemSummary, 0, len(ids))
	for _, batch := range ado.BatchIDs(ids) {
		idList := make([]string, len(batch))
		for i, n := range batch {
			idList[i] = strconv.Itoa(n)
		}
		urlStr := fmt.Sprintf("%s/_apis/wit/workitems?ids=%s&api-version=7.1",
			c.OrgURL(), strings.Join(idList, ","))
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		for _, wi := range resp.Value {
			out = append(out, transformWorkItemSummary(wi))
		}
	}
	return out, nil
}

// runWiql executes a WIQL query at the project scope and returns the matched IDs.
func runWiql(ctx context.Context, c *ado.Client, project, wiql string, top *int) ([]int, error) {
	q := url.Values{}
	q.Set("api-version", "7.1")
	if top != nil {
		q.Set("$top", strconv.Itoa(*top))
	}
	urlStr := fmt.Sprintf("%s/%s/_apis/wit/wiql?%s",
		c.OrgURL(), url.PathEscape(project), q.Encode())
	body := map[string]any{"query": wiql}
	var resp struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
	}
	if err := c.PostJSON(ctx, urlStr, body, &resp); err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(resp.WorkItems))
	for _, w := range resp.WorkItems {
		if w.ID != 0 {
			ids = append(ids, w.ID)
		}
	}
	return ids, nil
}

func configureWorkItemTools(s *server.MCPServer, c *ado.Client) {
	// get_work_item
	s.AddTool(mcp.NewTool("get_work_item",
		mcp.WithDescription("Get a single work item by ID with full details"),
		mcp.WithNumber("workItemId", mcp.Required(), mcp.Description("Work item ID")),
		mcp.WithArray("extraFields", mcp.WithStringItems(),
			mcp.Description("Additional ADO field reference names to include (e.g. Microsoft.VSTS.TCM.ReproSteps)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := reqInt(req, "workItemId")
		if err != nil {
			return nil, err
		}
		extra := req.GetStringSlice("extraFields", nil)
		urlStr := fmt.Sprintf("%s/_apis/wit/workitems/%d?$expand=all&api-version=7.1",
			c.OrgURL(), id)
		var wi map[string]any
		if err := c.GetJSON(ctx, urlStr, &wi); err != nil {
			return nil, err
		}
		return jsonResult(transformWorkItem(c.OrgURL(), wi, extra))
	}))

	// get_work_items_batch
	s.AddTool(mcp.NewTool("get_work_items_batch",
		mcp.WithDescription("Get multiple work items by IDs (summary view). Automatically handles batching for >200 IDs."),
		mcp.WithArray("workItemIds", mcp.WithNumberItems(), mcp.Required(),
			mcp.Description("Array of work item IDs")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		idsRaw, err := req.RequireIntSlice("workItemIds")
		if err != nil {
			return nil, err
		}
		items, err := fetchWorkItemsByIDs(ctx, c, idsRaw)
		if err != nil {
			return nil, err
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// list_work_item_comments
	s.AddTool(mcp.NewTool("list_work_item_comments",
		mcp.WithDescription("List comments on a work item"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithNumber("workItemId", mcp.Required(), mcp.Description("Work item ID")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		id, err := reqInt(req, "workItemId")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/wit/workItems/%d/comments?api-version=7.1-preview.4",
			c.OrgURL(), url.PathEscape(project), id)
		var resp struct {
			Comments []map[string]any `json:"comments"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimComment, 0, len(resp.Comments))
		for _, cm := range resp.Comments {
			version := 1
			if v, ok := cm["version"].(float64); ok {
				version = int(v)
			}
			cid := 0
			if v, ok := cm["id"].(float64); ok {
				cid = int(v)
			}
			items = append(items, slim.SlimComment{
				ID:          cid,
				Text:        slim.StripHtml(stringOf(cm["text"])),
				Author:      slim.FlattenIdentity(cm["createdBy"]),
				CreatedDate: slim.ToIsoDate(cm["createdDate"]),
				IsEdited:    version > 1,
			})
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// list_work_item_revisions
	s.AddTool(mcp.NewTool("list_work_item_revisions",
		mcp.WithDescription("Get revision history of a work item - shows only changed fields per revision (not full snapshots)"),
		mcp.WithNumber("workItemId", mcp.Required(), mcp.Description("Work item ID")),
		mcp.WithNumber("top", mcp.Description("Max revisions to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Revisions to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := reqInt(req, "workItemId")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/_apis/wit/workitems/%d/revisions?api-version=7.1",
			c.OrgURL(), id)
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		revs := resp.Value
		items := []slim.SlimRevision{}
		for i, curr := range revs {
			fields, _ := curr["fields"].(map[string]any)
			if fields == nil {
				fields = map[string]any{}
			}
			var prevFields map[string]any
			if i > 0 {
				prevFields, _ = revs[i-1]["fields"].(map[string]any)
			}
			if prevFields == nil {
				prevFields = map[string]any{}
			}

			fieldChanges := []slim.SlimFieldChange{}
			if i == 0 {
				keyFields := []string{"System.WorkItemType", "System.Title", "System.State", "System.AssignedTo"}
				for _, k := range keyFields {
					if v, ok := fields[k]; ok && v != nil {
						val := stringFieldRaw(v)
						fieldChanges = append(fieldChanges, slim.SlimFieldChange{
							Field:    slim.FriendlyFieldName(k),
							NewValue: val,
						})
					}
				}
			} else {
				keys := map[string]bool{}
				for k := range fields {
					keys[k] = true
				}
				for k := range prevFields {
					keys[k] = true
				}
				for k := range keys {
					if k == "System.Rev" || k == "System.Watermark" ||
						k == "System.AuthorizedDate" || k == "System.RevisedDate" {
						continue
					}
					currStr := stringFieldRaw(fields[k])
					prevStr := stringFieldRaw(prevFields[k])
					if currStr != prevStr {
						fc := slim.SlimFieldChange{Field: slim.FriendlyFieldName(k)}
						if prevStr != "" {
							fc.OldValue = prevStr
						}
						if currStr != "" {
							fc.NewValue = currStr
						}
						fieldChanges = append(fieldChanges, fc)
					}
				}
			}
			if len(fieldChanges) == 0 {
				continue
			}
			rev := i + 1
			if v, ok := curr["rev"].(float64); ok {
				rev = int(v)
			}
			items = append(items, slim.SlimRevision{
				Rev:          rev,
				ChangedBy:    slim.FlattenIdentity(fields["System.ChangedBy"]),
				ChangedDate:  slim.ToIsoDate(fields["System.ChangedDate"]),
				FieldChanges: fieldChanges,
			})
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// my_work_items
	s.AddTool(mcp.NewTool("my_work_items",
		mcp.WithDescription("List work items assigned to the current user"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		wiql := fmt.Sprintf(
			"SELECT [System.Id] FROM WorkItems WHERE [System.AssignedTo] = @Me AND [System.TeamProject] = '%s' AND [System.State] <> 'Closed' AND [System.State] <> 'Removed' ORDER BY [System.ChangedDate] DESC",
			project)
		ids, err := runWiql(ctx, c, project, wiql, nil)
		if err != nil {
			return nil, err
		}
		items, err := fetchWorkItemsByIDs(ctx, c, ids)
		if err != nil {
			return nil, err
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// get_work_items_for_iteration
	s.AddTool(mcp.NewTool("get_work_items_for_iteration",
		mcp.WithDescription("Get work items in a specific iteration/sprint"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("iterationPath", mcp.Required(), mcp.Description("Iteration path (e.g., 'Project\\Sprint 1')")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		iterPath, err := reqString(req, "iterationPath")
		if err != nil {
			return nil, err
		}
		wiql := fmt.Sprintf(
			"SELECT [System.Id] FROM WorkItems WHERE [System.IterationPath] = '%s' AND [System.TeamProject] = '%s' ORDER BY [Microsoft.VSTS.Common.Priority] ASC, [System.State] ASC",
			iterPath, project)
		ids, err := runWiql(ctx, c, project, wiql, nil)
		if err != nil {
			return nil, err
		}
		items, err := fetchWorkItemsByIDs(ctx, c, ids)
		if err != nil {
			return nil, err
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		end := sk + t
		if end > len(items) {
			end = len(items)
		}
		if sk > len(items) {
			sk = len(items)
		}
		out := slim.SlimIterationWorkItems{
			IterationPath: iterPath,
			WorkItems:     items[sk:end],
		}
		return jsonResult(out)
	}))

	// get_query
	s.AddTool(mcp.NewTool("get_query",
		mcp.WithDescription("Get a saved work item query definition by ID or path"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("queryId", mcp.Required(), mcp.Description("Query ID (GUID) or path (e.g., 'Shared Queries/Active Bugs')")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		queryID, err := reqString(req, "queryId")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/wit/queries/%s?$depth=1&api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(queryID))
		var q map[string]any
		if err := c.GetJSON(ctx, urlStr, &q); err != nil {
			return nil, err
		}
		out := slim.SlimQueryDefinition{
			ID:        stringOf(q["id"]),
			Name:      stringOf(q["name"]),
			Path:      stringOf(q["path"]),
			QueryType: "flat",
			Wiql:      stringOf(q["wiql"]),
			IsPublic:  asBool(q["isPublic"]),
		}
		if v, ok := q["queryType"]; ok && v != nil {
			out.QueryType = fmt.Sprint(v)
		}
		if cols, ok := q["columns"].([]any); ok {
			for _, col := range cols {
				cm, _ := col.(map[string]any)
				name := stringOf(cm["name"])
				if name == "" {
					name = stringOf(cm["referenceName"])
				}
				out.Columns = append(out.Columns, name)
			}
		}
		return jsonResult(out)
	}))

	// get_query_results
	s.AddTool(mcp.NewTool("get_query_results",
		mcp.WithDescription("Execute a saved query and return work item summaries"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("queryId", mcp.Required(), mcp.Description("Query ID (GUID)")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		queryID, err := reqString(req, "queryId")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/wit/wiql/%s?api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(queryID))
		var resp struct {
			WorkItems []struct {
				ID int `json:"id"`
			} `json:"workItems"`
		}
		if err := c.PostJSON(ctx, urlStr, map[string]any{}, &resp); err != nil {
			return nil, err
		}
		ids := make([]int, 0, len(resp.WorkItems))
		for _, w := range resp.WorkItems {
			if w.ID != 0 {
				ids = append(ids, w.ID)
			}
		}
		items, err := fetchWorkItemsByIDs(ctx, c, ids)
		if err != nil {
			return nil, err
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// run_wiql
	s.AddTool(mcp.NewTool("run_wiql",
		mcp.WithDescription("Execute an arbitrary WIQL query and return work item summaries"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("wiql", mcp.Required(), mcp.Description("WIQL query string")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		wiql, err := reqString(req, "wiql")
		if err != nil {
			return nil, err
		}
		topPtr := optInt(req, "top")
		ids, err := runWiql(ctx, c, project, wiql, topPtr)
		if err != nil {
			return nil, err
		}
		items, err := fetchWorkItemsByIDs(ctx, c, ids)
		if err != nil {
			return nil, err
		}
		// TS: explicit hasMore=false envelope.
		return jsonResult(map[string]any{"count": len(items), "hasMore": false, "items": items})
	}))

	// list_iterations
	s.AddTool(mcp.NewTool("list_iterations",
		mcp.WithDescription("List iterations (sprints) in a project to discover valid iteration paths"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithNumber("top", mcp.Description("Max items to return (default 50)")),
		mcp.WithNumber("skip", mcp.Description("Items to skip (default 0)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/wit/classificationnodes/Iterations?$depth=10&api-version=7.1",
			c.OrgURL(), url.PathEscape(project))
		var root map[string]any
		if err := c.GetJSON(ctx, urlStr, &root); err != nil {
			return nil, err
		}
		var items []slim.SlimIteration
		var walk func(node map[string]any, parent string)
		walk = func(node map[string]any, parent string) {
			name := stringOf(node["name"])
			path := name
			if parent != "" {
				path = parent + `\` + name
			}
			id := stringOf(node["identifier"])
			if id == "" {
				id = stringOf(node["id"])
			}
			it := slim.SlimIteration{ID: id, Name: name, Path: path}
			if attrs, ok := node["attributes"].(map[string]any); ok {
				if v, ok := attrs["startDate"]; ok && v != nil {
					it.StartDate = slim.ToIsoDate(v)
				}
				if v, ok := attrs["finishDate"]; ok && v != nil {
					it.EndDate = slim.ToIsoDate(v)
				}
			}
			items = append(items, it)
			if children, ok := node["children"].([]any); ok {
				for _, ch := range children {
					if cm, ok := ch.(map[string]any); ok {
						walk(cm, path)
					}
				}
			}
		}
		if root != nil {
			walk(root, "")
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
