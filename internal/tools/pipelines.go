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

var buildStatusMap = map[int]string{
	0:  "none",
	1:  "inProgress",
	2:  "completed",
	4:  "cancelling",
	8:  "postponed",
	32: "notStarted",
}

var buildResultMap = map[int]string{
	0: "succeeded",
	2: "partiallySucceeded",
	4: "failed",
	8: "canceled",
}

func mapBuildStatus(v any) string {
	if v == nil {
		return "unknown"
	}
	if n, ok := v.(float64); ok {
		if s, ok := buildStatusMap[int(n)]; ok {
			return s
		}
		return strconv.Itoa(int(n))
	}
	return fmt.Sprint(v)
}

func mapBuildResult(v any) string {
	if v == nil {
		return ""
	}
	if n, ok := v.(float64); ok {
		if s, ok := buildResultMap[int(n)]; ok {
			return s
		}
		return strconv.Itoa(int(n))
	}
	return fmt.Sprint(v)
}

func configurePipelineTools(s *server.MCPServer, c *ado.Client) {
	// list_pipeline_runs
	s.AddTool(mcp.NewTool("list_pipeline_runs",
		mcp.WithDescription("List recent pipeline/build runs"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithString("pipelineName", mcp.Description("Filter by pipeline/definition name")),
		mcp.WithNumber("top"),
		mcp.WithNumber("skip"),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		pipelineName := optString(req, "pipelineName")
		t, sk := ado.ResolveTopSkip(optInt(req, "top"), optInt(req, "skip"))

		var defIDs []string
		if pipelineName != "" {
			urlStr := fmt.Sprintf("%s/%s/_apis/build/definitions?name=%s&api-version=7.1",
				c.OrgURL(), url.PathEscape(project), url.QueryEscape(pipelineName))
			var resp struct {
				Value []map[string]any `json:"value"`
			}
			if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
				return nil, err
			}
			for _, d := range resp.Value {
				if id := intFromAny(d["id"]); id != 0 {
					defIDs = append(defIDs, strconv.Itoa(id))
				}
			}
			if len(defIDs) == 0 {
				return jsonResult(map[string]any{"count": 0, "hasMore": false, "items": []slim.SlimPipelineRun{}})
			}
		}

		q := url.Values{}
		q.Set("api-version", "7.1")
		q.Set("$top", strconv.Itoa(t))
		if len(defIDs) > 0 {
			q.Set("definitions", strings.Join(defIDs, ","))
		}
		urlStr := fmt.Sprintf("%s/%s/_apis/build/builds?%s",
			c.OrgURL(), url.PathEscape(project), q.Encode())
		var resp struct {
			Value []map[string]any `json:"value"`
		}
		if err := c.GetJSON(ctx, urlStr, &resp); err != nil {
			return nil, err
		}
		items := make([]slim.SlimPipelineRun, 0, len(resp.Value))
		for _, b := range resp.Value {
			defName := ""
			if d, ok := b["definition"].(map[string]any); ok {
				defName = stringOf(d["name"])
			}
			run := slim.SlimPipelineRun{
				RunID:        intFromAny(b["id"]),
				PipelineName: defName,
				Status:       mapBuildStatus(b["status"]),
				Result:       mapBuildResult(b["result"]),
				SourceBranch: slim.StripRefPrefix(stringOf(b["sourceBranch"])),
				RequestedBy:  slim.FlattenIdentity(b["requestedBy"]),
			}
			if v, ok := b["startTime"]; ok && v != nil {
				run.StartTime = slim.ToIsoDate(v)
			}
			if v, ok := b["finishTime"]; ok && v != nil {
				run.FinishTime = slim.ToIsoDate(v)
			}
			items = append(items, run)
		}
		// Manual skip slicing — TS quirk.
		if sk > len(items) {
			sk = len(items)
		}
		sliced := items[sk:]
		end := t
		if end > len(sliced) {
			end = len(sliced)
		}
		return jsonResult(map[string]any{
			"count":   end,
			"hasMore": len(sliced) > t,
			"items":   sliced[:end],
		})
	}))

	// get_pipeline_run
	s.AddTool(mcp.NewTool("get_pipeline_run",
		mcp.WithDescription("Get details of a specific pipeline/build run including stages"),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name or ID")),
		mcp.WithNumber("buildId", mcp.Required(), mcp.Description("Build/run ID")),
	), withError(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := reqString(req, "project")
		if err != nil {
			return nil, err
		}
		buildID, err := reqInt(req, "buildId")
		if err != nil {
			return nil, err
		}
		base := fmt.Sprintf("%s/%s/_apis/build/builds/%d",
			c.OrgURL(), url.PathEscape(project), buildID)
		var b map[string]any
		if err := c.GetJSON(ctx, base+"?api-version=7.1", &b); err != nil {
			return nil, err
		}
		var timeline struct {
			Records []map[string]any `json:"records"`
		}
		_ = c.GetJSON(ctx, base+"/timeline?api-version=7.1", &timeline)

		stages := []slim.SlimPipelineStage{}
		for _, r := range timeline.Records {
			if stringOf(r["type"]) != "Stage" {
				continue
			}
			st := slim.SlimPipelineStage{
				Name:   stringOf(r["name"]),
				Status: "unknown",
			}
			if v, ok := r["state"]; ok && v != nil {
				st.Status = fmt.Sprint(v)
			}
			if v, ok := r["result"]; ok && v != nil {
				st.Result = fmt.Sprint(v)
			}
			stages = append(stages, st)
		}
		defName := ""
		if d, ok := b["definition"].(map[string]any); ok {
			defName = stringOf(d["name"])
		}
		out := slim.SlimPipelineRunDetail{
			RunID:         intFromAny(b["id"]),
			PipelineName:  defName,
			Status:        mapBuildStatus(b["status"]),
			Result:        mapBuildResult(b["result"]),
			SourceBranch:  slim.StripRefPrefix(stringOf(b["sourceBranch"])),
			SourceVersion: slim.ShortSha(stringOf(b["sourceVersion"])),
			RequestedBy:   slim.FlattenIdentity(b["requestedBy"]),
			Stages:        stages,
			WebURL: fmt.Sprintf("%s/%s/_build/results?buildId=%d",
				c.OrgURL(), url.PathEscape(project), buildID),
		}
		if v, ok := b["startTime"]; ok && v != nil {
			out.StartTime = slim.ToIsoDate(v)
		}
		if v, ok := b["finishTime"]; ok && v != nil {
			out.FinishTime = slim.ToIsoDate(v)
		}
		return jsonResult(out)
	}))
}
