package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"ado-slim/internal/ado"
	"ado-slim/internal/slim"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var prStatusMap = []string{"unknown", "active", "abandoned", "completed"}

func mapPRStatus(v any) string {
	if v == nil {
		return "unknown"
	}
	if n, ok := v.(float64); ok {
		idx := int(n)
		if idx >= 0 && idx < len(prStatusMap) {
			return prStatusMap[idx]
		}
		return strconv.Itoa(idx)
	}
	return fmt.Sprint(v)
}

func mapThreadStatus(v any) string {
	if v == nil {
		return ""
	}
	n, ok := v.(float64)
	if !ok {
		return fmt.Sprint(v)
	}
	switch int(n) {
	case 0:
		return "unknown"
	case 1:
		return "active"
	case 2:
		return "fixed"
	case 3:
		return "wontFix"
	case 4:
		return "closed"
	case 5:
		return "byDesign"
	case 6:
		return "pending"
	default:
		return fmt.Sprintf("unknown(%d)", int(n))
	}
}

func intFromAny(v any) int {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	}
	return 0
}

func int64FromAny(v any) int64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	}
	return 0
}

func intPtrFromAny(v any) *int {
	if v == nil {
		return nil
	}
	if n, ok := v.(float64); ok {
		x := int(n)
		return &x
	}
	return nil
}

func configureRepositoryTools(s *server.MCPServer, c *ado.Client) {
	// list_repos
	s.AddTool(mcp.NewTool("list_repos",
		mcp.WithDescription("List all repositories in a project"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=7.1",
			c.OrgURL(), url.PathEscape(project))
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimRepoSummary, 0, len(resp.Value))
		for _, r := range resp.Value {
			items = append(items, slim.SlimRepoSummary{
				ID:            stringOf(r["id"]),
				Name:          stringOf(r["name"]),
				DefaultBranch: slim.StripRefPrefix(stringOf(r["defaultBranch"])),
				Size:          int64FromAny(r["size"]),
				IsDisabled:    asBool(r["isDisabled"]),
			})
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// get_repo
	s.AddTool(mcp.NewTool("get_repo",
		mcp.WithDescription("Get repository details"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s?api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo))
		var r map[string]any
		if err := c.GetJSON(ctx, urlStr, &r); err != nil {
			return nil, err
		}
		out := slim.SlimRepo{
			ID:            stringOf(r["id"]),
			Name:          stringOf(r["name"]),
			DefaultBranch: slim.StripRefPrefix(stringOf(r["defaultBranch"])),
			Size:          int64FromAny(r["size"]),
			WebURL:        stringOf(r["webUrl"]),
			IsDisabled:    asBool(r["isDisabled"]),
		}
		return jsonResult(out)
	}))

	// list_branches
	s.AddTool(mcp.NewTool("list_branches",
		mcp.WithDescription("List all branches in a repository"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/refs?filter=heads/&api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo))
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimBranch, 0, len(resp.Value))
		for _, b := range resp.Value {
			items = append(items, slim.SlimBranch{
				Name:     slim.StripRefPrefix(stringOf(b["name"])),
				CommitID: slim.ShortSha(stringOf(b["objectId"])),
			})
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// get_branch
	s.AddTool(mcp.NewTool("get_branch",
		mcp.WithDescription("Get branch details including latest commit and ahead/behind counts"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithString("branchName", mcp.Required(), mcp.Description("Branch name (without refs/heads/)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		branch, err := reqString(req, "branchName")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/stats/branches?name=%s&api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), url.QueryEscape(branch))
		var stats map[string]any
		if err := c.GetJSON(ctx, urlStr, &stats); err != nil {
			return nil, err
		}
		commit, _ := stats["commit"].(map[string]any)
		commitID := ""
		message := ""
		var authorRaw any
		var dateAny any
		if commit != nil {
			commitID = stringOf(commit["commitId"])
			message = slim.FirstLine(stringOf(commit["comment"]))
			authorRaw = commit["author"]
			if a, ok := authorRaw.(map[string]any); ok {
				dateAny = a["date"]
			}
		}
		out := slim.SlimBranchDetail{
			Name:        slim.StripRefPrefix(stringOf(stats["name"])),
			CommitID:    commitID,
			AheadCount:  intPtrFromAny(stats["aheadCount"]),
			BehindCount: intPtrFromAny(stats["behindCount"]),
			LatestCommit: slim.SlimBranchLatestCommit{
				Message: message,
				Author:  slim.FlattenIdentity(authorRaw),
				Date:    slim.ToIsoDate(dateAny),
			},
		}
		return jsonResult(out)
	}))

	// list_active_branches
	s.AddTool(mcp.NewTool("list_active_branches",
		mcp.WithDescription("List branches that received at least one push within a date range (and/or matching author/name filters). Backed by the ADO Pushes API."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithString("sinceDate", mcp.Description("Inclusive lower bound on push date (ISO 8601)")),
		mcp.WithString("untilDate", mcp.Description("Inclusive upper bound on push date (ISO 8601)")),
		mcp.WithString("authorContains", mcp.Description("Case-insensitive substring match on pusher display name")),
		mcp.WithString("namePattern", mcp.Description("Case-insensitive substring match on branch name")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		sinceDate := optString(req, "sinceDate")
		untilDate := optString(req, "untilDate")
		authorContains := optString(req, "authorContains")
		namePattern := optString(req, "namePattern")

		// Page through all pushes within the date range.
		const pageSize = 100
		pageSkip := 0
		type pushAgg struct {
			CommitID string
			Date     string
			Author   string
		}
		byRef := map[string]pushAgg{}
		for {
			q := url.Values{}
			q.Set("api-version", "7.1")
			q.Set("searchCriteria.includeRefUpdates", "true")
			if sinceDate != "" {
				q.Set("searchCriteria.fromDate", sinceDate)
			}
			if untilDate != "" {
				q.Set("searchCriteria.toDate", untilDate)
			}
			q.Set("$top", strconv.Itoa(pageSize))
			q.Set("$skip", strconv.Itoa(pageSkip))
			urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pushes?%s",
				c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), q.Encode())
			var resp struct {
				Value []map[string]any `json:"value"`
			}
			if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
				return nil, err
			}
			for _, push := range resp.Value {
				pushDate := slim.ToIsoDate(push["date"])
				author := ""
				if pb, ok := push["pushedBy"].(map[string]any); ok {
					author = stringOf(pb["displayName"])
				}
				refUpdates, _ := push["refUpdates"].([]any)
				for _, ruAny := range refUpdates {
					ru, ok := ruAny.(map[string]any)
					if !ok {
						continue
					}
					refName := stringOf(ru["name"])
					if !strings.HasPrefix(refName, "refs/heads/") {
						continue
					}
					existing, has := byRef[refName]
					if !has || pushDate > existing.Date {
						byRef[refName] = pushAgg{
							CommitID: slim.ShortSha(stringOf(ru["newObjectId"])),
							Date:     pushDate,
							Author:   author,
						}
					}
				}
			}
			if len(resp.Value) < pageSize {
				break
			}
			pageSkip += pageSize
		}

		items := make([]slim.SlimBranchActivity, 0, len(byRef))
		for refName, v := range byRef {
			items = append(items, slim.SlimBranchActivity{
				Name:     strings.TrimPrefix(refName, "refs/heads/"),
				CommitID: v.CommitID,
				LatestPush: slim.SlimBranchLatestPush{
					Date:   v.Date,
					Author: v.Author,
				},
			})
		}

		if namePattern != "" {
			np := strings.ToLower(namePattern)
			filtered := items[:0]
			for _, it := range items {
				if strings.Contains(strings.ToLower(it.Name), np) {
					filtered = append(filtered, it)
				}
			}
			items = filtered
		}
		if authorContains != "" {
			ac := strings.ToLower(authorContains)
			filtered := items[:0]
			for _, it := range items {
				if strings.Contains(strings.ToLower(it.LatestPush.Author), ac) {
					filtered = append(filtered, it)
				}
			}
			items = filtered
		}

		sort.SliceStable(items, func(i, j int) bool {
			return items[i].LatestPush.Date > items[j].LatestPush.Date
		})

		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		end := sk + t
		if sk > len(items) {
			sk = len(items)
		}
		if end > len(items) {
			end = len(items)
		}
		page := items[sk:end]
		return jsonResult(map[string]any{
			"count":   len(page),
			"hasMore": len(page) == t,
			"items":   page,
		})
	}))

	// search_commits
	s.AddTool(mcp.NewTool("search_commits",
		mcp.WithDescription("Search commits in a repository"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithString("author", mcp.Description("Filter by author")),
		mcp.WithString("fromDate", mcp.Description("From date (ISO format)")),
		mcp.WithString("toDate", mcp.Description("To date (ISO format)")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))

		q := url.Values{}
		q.Set("api-version", "7.1")
		q.Set("$top", strconv.Itoa(t))
		q.Set("$skip", strconv.Itoa(sk))
		if v := optString(req, "author"); v != "" {
			q.Set("searchCriteria.author", v)
		}
		if v := optString(req, "fromDate"); v != "" {
			q.Set("searchCriteria.fromDate", v)
		}
		if v := optString(req, "toDate"); v != "" {
			q.Set("searchCriteria.toDate", v)
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/commits?%s",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), q.Encode())
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimCommit, 0, len(resp.Value))
		for _, cm := range resp.Value {
			cc, _ := cm["changeCounts"].(map[string]any)
			ccGet := func(keys ...string) int {
				if cc == nil {
					return 0
				}
				for _, k := range keys {
					if v, ok := cc[k]; ok {
						return intFromAny(v)
					}
				}
				return 0
			}
			authorRaw := cm["author"]
			var dateAny any
			if a, ok := authorRaw.(map[string]any); ok {
				dateAny = a["date"]
			}
			items = append(items, slim.SlimCommit{
				CommitID: slim.ShortSha(stringOf(cm["commitId"])),
				Message:  slim.FirstLine(stringOf(cm["comment"])),
				Author:   slim.FlattenIdentity(authorRaw),
				Date:     slim.ToIsoDate(dateAny),
				ChangeCounts: slim.SlimChangeCounts{
					Add:    ccGet("Add", "add"),
					Edit:   ccGet("Edit", "edit"),
					Delete: ccGet("Delete", "delete"),
				},
			})
		}
		return jsonResult(map[string]any{
			"count":   len(items),
			"hasMore": len(items) == t,
			"items":   items,
		})
	}))

	// get_commit_changes
	s.AddTool(mcp.NewTool("get_commit_changes",
		mcp.WithDescription("Get the list of files changed in a specific commit"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithString("commitId", mcp.Required(), mcp.Description("Full commit SHA")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		commitID, err := reqString(req, "commitId")
		if err != nil {
			return nil, err
		}
		base := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/commits/%s",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), url.PathEscape(commitID))
		var changesResp struct {
			Changes []map[string]any `json:"changes"`
		}
		if err := c.GetJSON(ctx, base+"/changes?api-version=7.1", &changesResp); err != nil {
			return nil, err
		}
		var commit map[string]any
		if err := c.GetJSON(ctx, base+"?api-version=7.1", &commit); err != nil {
			return nil, err
		}
		authorRaw := commit["author"]
		var dateAny any
		if a, ok := authorRaw.(map[string]any); ok {
			dateAny = a["date"]
		}
		out := slim.SlimCommitChanges{
			CommitID: slim.ShortSha(commitID),
			Message:  slim.FirstLine(stringOf(commit["comment"])),
			Author:   slim.FlattenIdentity(authorRaw),
			Date:     slim.ToIsoDate(dateAny),
		}
		for _, ch := range changesResp.Changes {
			path := ""
			if it, ok := ch["item"].(map[string]any); ok {
				path = stringOf(it["path"])
			}
			out.Changes = append(out.Changes, slim.SlimCommitChange{
				Path:         path,
				ChangeType:   slim.MapChangeType(ch["changeType"]),
				OriginalPath: stringOf(ch["sourceServerItem"]),
			})
		}
		return jsonResult(out)
	}))

	// list_pull_requests
	s.AddTool(mcp.NewTool("list_pull_requests",
		mcp.WithDescription("List pull requests in a repository or project"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Description("Repository name or ID (omit for project-wide)")),
		mcp.WithString("status", mcp.Description("PR status filter (default: active)"),
			mcp.Enum("active", "completed", "abandoned", "all")),
		mcp.WithString("creatorId", mcp.Description("Filter by creator identity ID")),
		mcp.WithString("reviewerId", mcp.Description("Filter by reviewer identity ID")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo := optString(req, "repository")
		status := optString(req, "status")
		statusMap := map[string]int{"active": 1, "abandoned": 2, "completed": 3, "all": 4}
		statusN, ok := statusMap[status]
		if !ok {
			statusN = 1
		}

		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))

		q := url.Values{}
		q.Set("api-version", "7.1")
		q.Set("$top", strconv.Itoa(t))
		q.Set("$skip", strconv.Itoa(sk))
		q.Set("searchCriteria.status", strconv.Itoa(statusN))
		if v := optString(req, "creatorId"); v != "" {
			q.Set("searchCriteria.creatorId", v)
		}
		if v := optString(req, "reviewerId"); v != "" {
			q.Set("searchCriteria.reviewerId", v)
		}

		// Repository-scoped vs project-scoped routing.
		var urlStr string
		if repo != "" {
			urlStr = fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests?%s",
				c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), q.Encode())
		} else {
			urlStr = fmt.Sprintf("%s/%s/_apis/git/pullrequests?%s",
				c.OrgURL(), url.PathEscape(project), q.Encode())
		}
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimPullRequestSummary, 0, len(resp.Value))
		for _, pr := range resp.Value {
			reviewers := []slim.SlimReviewer{}
			if rs, ok := pr["reviewers"].([]any); ok {
				for _, r := range rs {
					reviewers = append(reviewers, slim.TransformReviewer(r))
				}
			}
			summary := slim.SlimPullRequestSummary{
				PullRequestID: intFromAny(pr["pullRequestId"]),
				Title:         stringOf(pr["title"]),
				Status:        mapPRStatus(pr["status"]),
				CreatedBy:     slim.FlattenIdentity(pr["createdBy"]),
				CreationDate:  slim.ToIsoDate(pr["creationDate"]),
				SourceBranch:  slim.StripRefPrefix(stringOf(pr["sourceRefName"])),
				TargetBranch:  slim.StripRefPrefix(stringOf(pr["targetRefName"])),
				Reviewers:     reviewers,
				IsDraft:       asBool(pr["isDraft"]),
			}
			if v, ok := pr["mergeStatus"]; ok && v != nil {
				summary.MergeStatus = fmt.Sprint(v)
			}
			if labels, ok := pr["labels"].([]any); ok {
				for _, l := range labels {
					if lm, ok := l.(map[string]any); ok {
						if name := stringOf(lm["name"]); name != "" {
							summary.Labels = append(summary.Labels, name)
						}
					}
				}
			}
			items = append(items, summary)
		}
		return jsonResult(map[string]any{
			"count":   len(items),
			"hasMore": len(items) == t,
			"items":   items,
		})
	}))

	// get_pull_request
	s.AddTool(mcp.NewTool("get_pull_request",
		mcp.WithDescription("Get full details of a pull request"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithNumber("pullRequestId", mcp.Required(), mcp.Description("Pull request ID")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		prID, err := reqInt(req, "pullRequestId")
		if err != nil {
			return nil, err
		}
		base := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), prID)
		var pr map[string]any
		if err := c.GetJSON(ctx, base+"?api-version=7.1", &pr); err != nil {
			return nil, err
		}
		var wiResp struct {
			Value []map[string]any `json:"value"`
		}
		// Best-effort: linked work items; tolerate failure.
		_ = c.GetJSON(ctx, base+"/workitems?api-version=7.1", &wiResp)
		workItemIDs := []int{}
		for _, ref := range wiResp.Value {
			if id, err := strconv.Atoi(stringOf(ref["id"])); err == nil {
				workItemIDs = append(workItemIDs, id)
			}
		}

		reviewers := []slim.SlimReviewer{}
		if rs, ok := pr["reviewers"].([]any); ok {
			for _, r := range rs {
				reviewers = append(reviewers, slim.TransformReviewer(r))
			}
		}

		desc := stringOf(pr["description"])
		if len(desc) > 3000 {
			desc = desc[:3000]
		}

		out := slim.SlimPullRequest{
			PullRequestID: intFromAny(pr["pullRequestId"]),
			Title:         stringOf(pr["title"]),
			Description:   desc,
			Status:        mapPRStatus(pr["status"]),
			CreatedBy:     slim.FlattenIdentity(pr["createdBy"]),
			CreationDate:  slim.ToIsoDate(pr["creationDate"]),
			SourceBranch:  slim.StripRefPrefix(stringOf(pr["sourceRefName"])),
			TargetBranch:  slim.StripRefPrefix(stringOf(pr["targetRefName"])),
			Reviewers:     reviewers,
			IsDraft:       asBool(pr["isDraft"]),
			WorkItemIDs:   workItemIDs,
			WebURL: fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d",
				c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), prID),
		}
		if v, ok := pr["closedDate"]; ok && v != nil {
			out.ClosedDate = slim.ToIsoDate(v)
		}
		if v, ok := pr["mergeStatus"]; ok && v != nil {
			out.MergeStatus = fmt.Sprint(v)
		}
		if labels, ok := pr["labels"].([]any); ok {
			for _, l := range labels {
				if lm, ok := l.(map[string]any); ok {
					if name := stringOf(lm["name"]); name != "" {
						out.Labels = append(out.Labels, name)
					}
				}
			}
		}
		if co, ok := pr["completionOptions"].(map[string]any); ok && co != nil {
			out.CompletionOptions = &slim.SlimCompletionOptions{
				MergeStrategy:      stringOf(co["mergeStrategy"]),
				DeleteSourceBranch: asBool(co["deleteSourceBranch"]),
			}
		}
		if v, ok := pr["autoCompleteSetBy"]; ok && v != nil {
			out.AutoCompleteSetBy = slim.FlattenIdentity(v)
		}
		return jsonResult(out)
	}))

	// get_pull_request_changes
	s.AddTool(mcp.NewTool("get_pull_request_changes",
		mcp.WithDescription("Get the list of files changed in a pull request"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithNumber("pullRequestId", mcp.Required(), mcp.Description("Pull request ID")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		prID, err := reqInt(req, "pullRequestId")
		if err != nil {
			return nil, err
		}
		base := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), prID)
		var iters struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, base+"/iterations?api-version=7.1", &iters); err != nil {
			return nil, err
		}
		if len(iters.Value) == 0 {
			return jsonResult(slim.SlimPullRequestChanges{
				PullRequestID: prID, ChangeCount: 0, Changes: []slim.SlimPRFileChange{},
			})
		}
		latest := iters.Value[len(iters.Value)-1]
		iterID := intFromAny(latest["id"])
		var changesResp struct {
			ChangeEntries []map[string]any `json:"changeEntries"`
		}
		if err := c.GetJSON(ctx, fmt.Sprintf("%s/iterations/%d/changes?api-version=7.1", base, iterID), &changesResp); err != nil {
			return nil, err
		}
		out := slim.SlimPullRequestChanges{PullRequestID: prID}
		for _, ce := range changesResp.ChangeEntries {
			path := ""
			if it, ok := ce["item"].(map[string]any); ok {
				path = stringOf(it["path"])
			}
			out.Changes = append(out.Changes, slim.SlimPRFileChange{
				Path:         path,
				ChangeType:   slim.MapChangeType(ce["changeType"]),
				OriginalPath: stringOf(ce["originalPath"]),
			})
		}
		out.ChangeCount = len(out.Changes)
		return jsonResult(out)
	}))

	// list_pull_request_threads
	s.AddTool(mcp.NewTool("list_pull_request_threads",
		mcp.WithDescription("List comment threads on a pull request"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithNumber("pullRequestId", mcp.Required(), mcp.Description("Pull request ID")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		prID, err := reqInt(req, "pullRequestId")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/threads?api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), prID)
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := []slim.SlimThread{}
		for _, th := range resp.Value {
			if asBool(th["isDeleted"]) {
				continue
			}
			comments, _ := th["comments"].([]any)
			var first map[string]any
			if len(comments) > 0 {
				first, _ = comments[0].(map[string]any)
			}
			ctxRaw, _ := th["threadContext"].(map[string]any)
			thread := slim.SlimThread{
				ThreadID:        intFromAny(th["id"]),
				IsDeleted:       false,
				CommentCount:    len(comments),
				LastUpdatedDate: slim.ToIsoDate(th["lastUpdatedDate"]),
			}
			if v, ok := th["status"]; ok && v != nil {
				thread.Status = mapThreadStatus(v)
			}
			if ctxRaw != nil {
				if fp := stringOf(ctxRaw["filePath"]); fp != "" {
					thread.FilePath = fp
				}
				rs, hasRS := ctxRaw["rightFileStart"].(map[string]any)
				re, hasRE := ctxRaw["rightFileEnd"].(map[string]any)
				if hasRS && hasRE {
					thread.LineRange = &slim.SlimLineRange{
						Start: intFromAny(rs["line"]),
						End:   intFromAny(re["line"]),
					}
				}
			}
			if first != nil {
				thread.FirstComment = slim.SlimThreadFirstComment{
					Author: slim.FlattenIdentity(first["author"]),
					Text:   slim.StripHtml(stringOf(first["content"])),
					Date:   slim.ToIsoDate(first["publishedDate"]),
				}
			} else {
				thread.FirstComment = slim.SlimThreadFirstComment{Author: "Unknown"}
			}
			items = append(items, thread)
		}
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))
		return jsonResult(ado.Paginate(items, t, sk))
	}))

	// list_pull_request_thread_comments
	s.AddTool(mcp.NewTool("list_pull_request_thread_comments",
		mcp.WithDescription("List comments in a specific PR thread"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithNumber("pullRequestId", mcp.Required(), mcp.Description("Pull request ID")),
		mcp.WithNumber("threadId", mcp.Required(), mcp.Description("Thread ID")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		prID, err := reqInt(req, "pullRequestId")
		if err != nil {
			return nil, err
		}
		threadID, err := reqInt(req, "threadId")
		if err != nil {
			return nil, err
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d/threads/%d/comments?api-version=7.1",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), prID, threadID)
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := []slim.SlimThreadComment{}
		for _, cm := range resp.Value {
			if asBool(cm["isDeleted"]) {
				continue
			}
			cmType := "text"
			if v, ok := cm["commentType"].(float64); ok && int(v) == 2 {
				cmType = "system"
			}
			items = append(items, slim.SlimThreadComment{
				CommentID:       intFromAny(cm["id"]),
				ParentCommentID: intFromAny(cm["parentCommentId"]),
				Author:          slim.FlattenIdentity(cm["author"]),
				Text:            slim.StripHtml(stringOf(cm["content"])),
				Date:            slim.ToIsoDate(cm["publishedDate"]),
				IsEdited:        stringOf(cm["publishedDate"]) != stringOf(cm["lastContentUpdatedDate"]),
				CommentType:     cmType,
			})
		}
		return jsonResult(map[string]any{"count": len(items), "hasMore": false, "items": items})
	}))

	// list_directory
	s.AddTool(mcp.NewTool("list_directory",
		mcp.WithDescription("List files and folders at a path in a repository"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithString("path", mcp.Description("Path in the repo (default: root '/')")),
		mcp.WithString("branch", mcp.Description("Branch name (default: repo default branch)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		path := optString(req, "path")
		if path == "" {
			path = "/"
		}
		branch := optString(req, "branch")

		q := url.Values{}
		q.Set("scopePath", path)
		q.Set("recursionLevel", "OneLevel")
		q.Set("includeContentMetadata", "true")
		if branch != "" {
			q.Set("versionDescriptor.version", branch)
			q.Set("versionDescriptor.versionType", "branch")
		}
		q.Set("api-version", "7.1")
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items?%s",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), q.Encode())
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		entries := []slim.SlimDirectoryEntry{}
		for _, it := range resp.Value {
			ip := stringOf(it["path"])
			if ip == path {
				continue
			}
			isFolder := asBool(it["isFolder"])
			t := "file"
			if isFolder {
				t = "folder"
			}
			entry := slim.SlimDirectoryEntry{Path: ip, Type: t}
			if !isFolder {
				if sz, ok := it["size"].(float64); ok {
					n := int64(sz)
					entry.Size = &n
				}
			}
			entries = append(entries, entry)
		}
		return jsonResult(map[string]any{
			"count": len(entries), "hasMore": false, "items": entries,
		})
	}))

	// get_file_content
	s.AddTool(mcp.NewTool("get_file_content",
		mcp.WithDescription("Read file content from a repository"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("repository", mcp.Required(), mcp.Description("Repository name or ID")),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path in the repo")),
		mcp.WithString("branch", mcp.Description("Branch name (default: repo default branch)")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		repo, err := reqString(req, "repository")
		if err != nil {
			return nil, err
		}
		path, err := reqString(req, "path")
		if err != nil {
			return nil, err
		}
		branch := optString(req, "branch")

		q := url.Values{}
		q.Set("path", path)
		if branch != "" {
			q.Set("versionDescriptor.version", branch)
			q.Set("versionDescriptor.versionType", "branch")
		}
		q.Set("includeContent", "true")
		q.Set("api-version", "7.1")
		urlStr := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items?%s",
			c.OrgURL(), url.PathEscape(project), url.PathEscape(repo), q.Encode())

		data, _, err := c.GetRaw(ctx, urlStr, "application/octet-stream")
		if err != nil {
			return nil, err
		}
		content := string(data)
		// JSON-error-as-content detection (mirrors TS).
		if (len(content) >= 7 && content[:7] == `{"$id":`) ||
			(len(content) >= 11 && content[:11] == `{"typeName"`) {
			var errObj map[string]any
			if json.Unmarshal(data, &errObj) == nil {
				msg := stringOf(errObj["message"])
				if msg == "" {
					msg = stringOf(errObj["typeKey"])
				}
				if msg == "" {
					msg = "Unknown error"
				}
				return mcp.NewToolResultError(fmt.Sprintf("Error reading file: %s", msg)), nil
			}
		}
		// Binary detection.
		if bytes.IndexByte(data, 0) >= 0 {
			return jsonResult(map[string]any{
				"path":    path,
				"content": "[Binary file - content not displayed]",
				"size":    len(content),
			})
		}
		return jsonResult(slim.SlimFileContent{
			Path:    path,
			Content: content,
			Size:    len(content),
		})
	}))
}
