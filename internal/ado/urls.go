package ado

import (
	"net/url"
	"strings"
)

// OrgURL returns the canonical base URL for the given Azure DevOps
// organization, e.g. "https://dev.azure.com/{org}".
func OrgURL(org string) string {
	return "https://dev.azure.com/" + url.PathEscape(org)
}

// SearchOrgURL returns the canonical base URL for the Search API host, which
// is *different* from the main ADO host: "https://almsearch.dev.azure.com/{org}".
func SearchOrgURL(org string) string {
	return "https://almsearch.dev.azure.com/" + url.PathEscape(org)
}

// JoinPath joins URL path segments with a single "/" separator. Each segment
// is path-escaped. The base may already contain a trailing slash; it is
// normalized away. Empty segments are skipped.
func JoinPath(base string, segments ...string) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "/"))
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		b.WriteByte('/')
		b.WriteString(url.PathEscape(seg))
	}
	return b.String()
}
