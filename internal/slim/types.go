// Package slim contains the DTO struct types and pure transform functions
// that turn raw Azure DevOps REST payloads into the slim representations
// returned to MCP clients. Mirrors src/types.ts and src/transforms.ts.
package slim

// Identity is the flattened "Display Name <email>" string form.
type Identity = string

// === Core ===

type SlimProject struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	State          string `json:"state"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type SlimTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SlimIdentityLookup struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// === Work Items ===

type SlimWorkItem struct {
	ID                int               `json:"id"`
	Type              string            `json:"type"`
	Title             string            `json:"title"`
	State             string            `json:"state"`
	Reason            string            `json:"reason"`
	AssignedTo        string            `json:"assignedTo,omitempty"`
	CreatedBy         string            `json:"createdBy"`
	CreatedDate       string            `json:"createdDate"`
	ChangedDate       string            `json:"changedDate"`
	ResolvedBy        string            `json:"resolvedBy,omitempty"`
	ResolvedDate      string            `json:"resolvedDate,omitempty"`
	ClosedBy          string            `json:"closedBy,omitempty"`
	ClosedDate        string            `json:"closedDate,omitempty"`
	Priority          *float64          `json:"priority,omitempty"`
	Severity          string            `json:"severity,omitempty"`
	IterationPath     string            `json:"iterationPath"`
	AreaPath          string            `json:"areaPath"`
	Tags              string            `json:"tags,omitempty"`
	StoryPoints       *float64          `json:"storyPoints,omitempty"`
	Relations         []SlimRelation    `json:"relations"`
	Description       string            `json:"description,omitempty"`
	AcceptanceCriteria string           `json:"acceptanceCriteria,omitempty"`
	ExtraFields       map[string]string `json:"extraFields,omitempty"`
	RemainingWork     *float64          `json:"remainingWork,omitempty"`
	OriginalEstimate  *float64          `json:"originalEstimate,omitempty"`
	CompletedWork     *float64          `json:"completedWork,omitempty"`
	WebURL            string            `json:"webUrl"`
}

type SlimRelation struct {
	Type     string `json:"type"`
	TargetID int    `json:"targetId"`
}

type SlimWorkItemSummary struct {
	ID            int      `json:"id"`
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	State         string   `json:"state"`
	AssignedTo    string   `json:"assignedTo,omitempty"`
	Priority      *float64 `json:"priority,omitempty"`
	StoryPoints   *float64 `json:"storyPoints,omitempty"`
	IterationPath string   `json:"iterationPath"`
	AreaPath      string   `json:"areaPath"`
	Tags          string   `json:"tags,omitempty"`
}

type SlimComment struct {
	ID          int    `json:"id"`
	Text        string `json:"text"`
	Author      string `json:"author"`
	CreatedDate string `json:"createdDate"`
	IsEdited    bool   `json:"isEdited"`
}

type SlimRevision struct {
	Rev          int              `json:"rev"`
	ChangedBy    string           `json:"changedBy"`
	ChangedDate  string           `json:"changedDate"`
	FieldChanges []SlimFieldChange `json:"fieldChanges"`
}

type SlimFieldChange struct {
	Field    string `json:"field"`
	OldValue string `json:"oldValue,omitempty"`
	NewValue string `json:"newValue,omitempty"`
}

type SlimIterationWorkItems struct {
	IterationPath string                `json:"iterationPath"`
	StartDate     string                `json:"startDate,omitempty"`
	EndDate       string                `json:"endDate,omitempty"`
	WorkItems     []SlimWorkItemSummary `json:"workItems"`
}

type SlimIteration struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

type SlimQueryDefinition struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	QueryType string   `json:"queryType"`
	Wiql      string   `json:"wiql,omitempty"`
	Columns   []string `json:"columns,omitempty"`
	IsPublic  bool     `json:"isPublic"`
}

// === Repositories ===

type SlimRepoSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
	Size          int64  `json:"size"`
	IsDisabled    bool   `json:"isDisabled"`
}

type SlimRepo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
	Size          int64  `json:"size"`
	WebURL        string `json:"webUrl"`
	IsDisabled    bool   `json:"isDisabled"`
}

type SlimBranch struct {
	Name     string `json:"name"`
	CommitID string `json:"commitId"`
}

type SlimBranchLatestCommit struct {
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

type SlimBranchDetail struct {
	Name         string                 `json:"name"`
	CommitID     string                 `json:"commitId"`
	AheadCount   *int                   `json:"aheadCount,omitempty"`
	BehindCount  *int                   `json:"behindCount,omitempty"`
	LatestCommit SlimBranchLatestCommit `json:"latestCommit"`
}

type SlimBranchLatestPush struct {
	Date   string `json:"date"`
	Author string `json:"author"`
}

type SlimBranchActivity struct {
	Name       string               `json:"name"`
	CommitID   string               `json:"commitId"`
	LatestPush SlimBranchLatestPush `json:"latestPush"`
}

type SlimChangeCounts struct {
	Add    int `json:"add"`
	Edit   int `json:"edit"`
	Delete int `json:"delete"`
}

type SlimCommit struct {
	CommitID     string           `json:"commitId"`
	Message      string           `json:"message"`
	Author       string           `json:"author"`
	Date         string           `json:"date"`
	ChangeCounts SlimChangeCounts `json:"changeCounts"`
}

type SlimCommitChange struct {
	Path         string `json:"path"`
	ChangeType   string `json:"changeType"`
	OriginalPath string `json:"originalPath,omitempty"`
}

type SlimCommitChanges struct {
	CommitID string             `json:"commitId"`
	Message  string             `json:"message"`
	Author   string             `json:"author"`
	Date     string             `json:"date"`
	Changes  []SlimCommitChange `json:"changes"`
}

type SlimReviewer struct {
	Name       string `json:"name"`
	Vote       string `json:"vote"`
	IsRequired bool   `json:"isRequired"`
}

type SlimPullRequestSummary struct {
	PullRequestID int            `json:"pullRequestId"`
	Title         string         `json:"title"`
	Status        string         `json:"status"`
	CreatedBy     string         `json:"createdBy"`
	CreationDate  string         `json:"creationDate"`
	SourceBranch  string         `json:"sourceBranch"`
	TargetBranch  string         `json:"targetBranch"`
	Reviewers     []SlimReviewer `json:"reviewers"`
	IsDraft       bool           `json:"isDraft"`
	MergeStatus   string         `json:"mergeStatus,omitempty"`
	Labels        []string       `json:"labels,omitempty"`
}

type SlimCompletionOptions struct {
	MergeStrategy      string `json:"mergeStrategy"`
	DeleteSourceBranch bool   `json:"deleteSourceBranch"`
}

type SlimPullRequest struct {
	PullRequestID      int                    `json:"pullRequestId"`
	Title              string                 `json:"title"`
	Description        string                 `json:"description,omitempty"`
	Status             string                 `json:"status"`
	CreatedBy          string                 `json:"createdBy"`
	CreationDate       string                 `json:"creationDate"`
	ClosedDate         string                 `json:"closedDate,omitempty"`
	SourceBranch       string                 `json:"sourceBranch"`
	TargetBranch       string                 `json:"targetBranch"`
	Reviewers          []SlimReviewer         `json:"reviewers"`
	IsDraft            bool                   `json:"isDraft"`
	MergeStatus        string                 `json:"mergeStatus,omitempty"`
	Labels             []string               `json:"labels,omitempty"`
	WorkItemIDs        []int                  `json:"workItemIds"`
	CompletionOptions  *SlimCompletionOptions `json:"completionOptions,omitempty"`
	AutoCompleteSetBy  string                 `json:"autoCompleteSetBy,omitempty"`
	WebURL             string                 `json:"webUrl"`
}

type SlimPRFileChange struct {
	Path         string `json:"path"`
	ChangeType   string `json:"changeType"`
	OriginalPath string `json:"originalPath,omitempty"`
}

type SlimPullRequestChanges struct {
	PullRequestID int                `json:"pullRequestId"`
	ChangeCount   int                `json:"changeCount"`
	Changes       []SlimPRFileChange `json:"changes"`
}

type SlimLineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type SlimThreadFirstComment struct {
	Author string `json:"author"`
	Text   string `json:"text"`
	Date   string `json:"date"`
}

type SlimThread struct {
	ThreadID        int                    `json:"threadId"`
	Status          string                 `json:"status,omitempty"`
	FilePath        string                 `json:"filePath,omitempty"`
	LineRange       *SlimLineRange         `json:"lineRange,omitempty"`
	IsDeleted       bool                   `json:"isDeleted"`
	CommentCount    int                    `json:"commentCount"`
	LastUpdatedDate string                 `json:"lastUpdatedDate"`
	FirstComment    SlimThreadFirstComment `json:"firstComment"`
}

type SlimThreadComment struct {
	CommentID       int    `json:"commentId"`
	ParentCommentID int    `json:"parentCommentId"`
	Author          string `json:"author"`
	Text            string `json:"text"`
	Date            string `json:"date"`
	IsEdited        bool   `json:"isEdited"`
	CommentType     string `json:"commentType"`
}

type SlimDirectoryEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" | "folder"
	Size *int64 `json:"size,omitempty"`
}

type SlimFileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// === Pipelines ===

type SlimPipelineRun struct {
	RunID        int    `json:"runId"`
	PipelineName string `json:"pipelineName"`
	Status       string `json:"status"`
	Result       string `json:"result,omitempty"`
	SourceBranch string `json:"sourceBranch"`
	StartTime    string `json:"startTime,omitempty"`
	FinishTime   string `json:"finishTime,omitempty"`
	RequestedBy  string `json:"requestedBy"`
}

type SlimPipelineStage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

type SlimPipelineRunDetail struct {
	RunID         int                 `json:"runId"`
	PipelineName  string              `json:"pipelineName"`
	Status        string              `json:"status"`
	Result        string              `json:"result,omitempty"`
	SourceBranch  string              `json:"sourceBranch"`
	SourceVersion string              `json:"sourceVersion"`
	StartTime     string              `json:"startTime,omitempty"`
	FinishTime    string              `json:"finishTime,omitempty"`
	RequestedBy   string              `json:"requestedBy"`
	Stages        []SlimPipelineStage `json:"stages"`
	WebURL        string              `json:"webUrl"`
}

// === Search ===

type SlimSearchMatch struct {
	Field    string   `json:"field"`
	Snippets []string `json:"snippets"`
}

type SlimCodeSearchResult struct {
	FileName string            `json:"fileName"`
	Path     string            `json:"path"`
	Repo     string            `json:"repo"`
	Project  string            `json:"project"`
	Branch   string            `json:"branch"`
	Matches  []SlimSearchMatch `json:"matches"`
}

type SlimWikiSearchResult struct {
	Title    string `json:"title"`
	Path     string `json:"path"`
	Project  string `json:"project"`
	WikiName string `json:"wikiName"`
	Snippet  string `json:"snippet"`
}

type SlimWorkItemSearchResult struct {
	ID            int      `json:"id"`
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	State         string   `json:"state"`
	AssignedTo    string   `json:"assignedTo,omitempty"`
	Project       string   `json:"project"`
	Snippet       string   `json:"snippet"`
	MatchedFields []string `json:"matchedFields"`
}
