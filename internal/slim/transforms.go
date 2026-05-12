package slim

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// === Identity Flattening ===

// FlattenIdentity reduces an ADO identity reference to "Display Name <email>"
// or just "Display Name" when no usable email exists. Mirrors flattenIdentity
// from src/transforms.ts.
func FlattenIdentity(v any) Identity {
	if v == nil {
		return "Unknown"
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		name := stringField(t, "displayName")
		if name == "" {
			name = stringField(t, "name")
		}
		if name == "" {
			name = "Unknown"
		}
		email := stringField(t, "uniqueName")
		if email == "" {
			email = stringField(t, "emailAddress")
		}
		if email == "" || email == name {
			return name
		}
		return fmt.Sprintf("%s <%s>", name, email)
	default:
		return "Unknown"
	}
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// === HTML Stripping ===

var (
	reBlockTags  = regexp.MustCompile(`(?i)</(div|p|li|tr|br)\s*>`)
	reBrSelf     = regexp.MustCompile(`(?i)<br\s*/?>`)
	reLiOpen     = regexp.MustCompile(`(?i)<li\s*>`)
	reAnyTag     = regexp.MustCompile(`<[^>]+>`)
	reMultiBlank = regexp.MustCompile(`\n{3,}`)
)

// StripHtml removes HTML tags and decodes a small set of entities,
// preserving block-level newlines and `<li>` bullets. Mirrors stripHtml
// from src/transforms.ts.
func StripHtml(html string) string {
	if html == "" {
		return ""
	}
	s := reBlockTags.ReplaceAllString(html, "\n")
	s = reBrSelf.ReplaceAllString(s, "\n")
	s = reLiOpen.ReplaceAllString(s, "- ")
	s = reAnyTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = reMultiBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// === Branch Ref Stripping ===

// StripRefPrefix removes the "refs/heads/" prefix if present.
func StripRefPrefix(ref string) string {
	if ref == "" {
		return ""
	}
	return strings.TrimPrefix(ref, "refs/heads/")
}

// === Vote Mapping ===

var voteMap = map[int]string{
	-10: "rejected",
	-5:  "waitForAuthor",
	0:   "noVote",
	5:   "approvedWithSuggestions",
	10:  "approved",
}

// MapVote translates a numeric reviewer vote to a readable string.
func MapVote(vote any) string {
	if vote == nil {
		return "noVote"
	}
	n, ok := toInt(vote)
	if !ok {
		return "noVote"
	}
	if s, ok := voteMap[n]; ok {
		return s
	}
	return fmt.Sprintf("unknown(%d)", n)
}

// TransformReviewer turns a raw reviewer object into a SlimReviewer.
func TransformReviewer(r any) SlimReviewer {
	m, _ := r.(map[string]any)
	required := false
	if v, ok := m["isRequired"].(bool); ok {
		required = v
	}
	return SlimReviewer{
		Name:       FlattenIdentity(r),
		Vote:       MapVote(m["vote"]),
		IsRequired: required,
	}
}

// === Field Name Mapping ===

var fieldNameMap = map[string]string{
	"System.Title":                                 "Title",
	"System.State":                                 "State",
	"System.Reason":                                "Reason",
	"System.AssignedTo":                            "Assigned To",
	"System.CreatedBy":                             "Created By",
	"System.CreatedDate":                           "Created Date",
	"System.ChangedBy":                             "Changed By",
	"System.ChangedDate":                           "Changed Date",
	"System.IterationPath":                         "Iteration Path",
	"System.AreaPath":                              "Area Path",
	"System.Tags":                                  "Tags",
	"System.Description":                           "Description",
	"System.WorkItemType":                          "Work Item Type",
	"Microsoft.VSTS.Common.Priority":               "Priority",
	"Microsoft.VSTS.Common.Severity":               "Severity",
	"Microsoft.VSTS.Common.ResolvedBy":             "Resolved By",
	"Microsoft.VSTS.Common.ResolvedDate":           "Resolved Date",
	"Microsoft.VSTS.Common.ClosedBy":               "Closed By",
	"Microsoft.VSTS.Common.ClosedDate":             "Closed Date",
	"Microsoft.VSTS.Common.AcceptanceCriteria":     "Acceptance Criteria",
	"Microsoft.VSTS.Common.StateChangeDate":        "State Change Date",
	"Microsoft.VSTS.Scheduling.RemainingWork":      "Remaining Work",
	"Microsoft.VSTS.Scheduling.OriginalEstimate":   "Original Estimate",
	"Microsoft.VSTS.Scheduling.CompletedWork":      "Completed Work",
	"Microsoft.VSTS.Scheduling.StoryPoints":        "Story Points",
}

var reFieldStrip = regexp.MustCompile(`^(System\.|Microsoft\.VSTS\.\w+\.)`)

// FriendlyFieldName returns the user-friendly label for an ADO field
// reference name. Unknown names are stripped of their System./Microsoft.*
// prefix.
func FriendlyFieldName(refName string) string {
	if v, ok := fieldNameMap[refName]; ok {
		return v
	}
	return reFieldStrip.ReplaceAllString(refName, "")
}

// === Relation Type Mapping ===

var relationTypeMap = map[string]string{
	"System.LinkTypes.Hierarchy-Forward":         "Child",
	"System.LinkTypes.Hierarchy-Reverse":         "Parent",
	"System.LinkTypes.Related":                   "Related",
	"Microsoft.VSTS.Common.TestedBy-Forward":     "Tested By",
	"Microsoft.VSTS.Common.TestedBy-Reverse":     "Tests",
	"System.LinkTypes.Dependency-Forward":        "Successor",
	"System.LinkTypes.Dependency-Reverse":        "Predecessor",
	"System.LinkTypes.Duplicate-Forward":         "Duplicate",
	"System.LinkTypes.Duplicate-Reverse":         "Duplicate Of",
}

// FriendlyRelationType returns the friendly name for a relation, or the input.
func FriendlyRelationType(rel string) string {
	if v, ok := relationTypeMap[rel]; ok {
		return v
	}
	return rel
}

var reWorkItemURL = regexp.MustCompile(`workItems/(\d+)$`)

// ExtractWorkItemIDFromURL parses the trailing /workItems/<id> off a URL.
// Returns -1 when no match.
func ExtractWorkItemIDFromURL(u string) int {
	m := reWorkItemURL.FindStringSubmatch(u)
	if m == nil {
		return -1
	}
	n := 0
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	return n
}

// === Commit Message ===

// FirstLine returns the first line of a (possibly multi-line) commit message.
func FirstLine(msg string) string {
	if msg == "" {
		return ""
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}
	return strings.TrimSpace(msg)
}

// ShortSha truncates a commit SHA to 8 characters.
func ShortSha(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}

// === Change Type Mapping ===

var changeTypeMap = map[int]string{
	1:    "add",
	2:    "edit",
	4:    "encoding",
	8:    "rename",
	16:   "delete",
	32:   "undelete",
	64:   "branch",
	128:  "merge",
	256:  "lock",
	512:  "rollback",
	1024: "sourceRename",
	2048: "targetRename",
}

// MapChangeType maps a numeric or already-string ADO change type to a slug.
func MapChangeType(v any) string {
	if v == nil {
		return "unknown"
	}
	if s, ok := v.(string); ok {
		return strings.ToLower(s)
	}
	if n, ok := toInt(v); ok {
		if s, ok := changeTypeMap[n]; ok {
			return s
		}
		return fmt.Sprintf("unknown(%d)", n)
	}
	return "unknown"
}

// === ISO Date ===

// ToIsoDate normalizes an arbitrary date input to an ISO-8601 string.
func ToIsoDate(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(v)
	}
}

// === helpers ===

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case string:
		// best-effort
		x := 0
		neg := false
		i := 0
		if len(n) > 0 && (n[0] == '-' || n[0] == '+') {
			neg = n[0] == '-'
			i = 1
		}
		if i == len(n) {
			return 0, false
		}
		for ; i < len(n); i++ {
			c := n[i]
			if c < '0' || c > '9' {
				return 0, false
			}
			x = x*10 + int(c-'0')
		}
		if neg {
			x = -x
		}
		return x, true
	}
	return 0, false
}
