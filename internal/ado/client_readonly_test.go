package ado

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", raw, err)
	}
	return u
}

func TestReadOnlyAllowed_GETAlwaysAllowed(t *testing.T) {
	for _, raw := range []string{
		"https://dev.azure.com/x/_apis/projects",
		"https://almsearch.dev.azure.com/x/_apis/search/codesearchresults",
		"https://example.com/anything",
	} {
		if err := readOnlyAllowed(http.MethodGet, mustURL(t, raw)); err != nil {
			t.Fatalf("GET %s rejected: %v", raw, err)
		}
	}
}

func TestReadOnlyAllowed_POSTAllowlist(t *testing.T) {
	for _, raw := range []string{
		"https://almsearch.dev.azure.com/x/_apis/search/codesearchresults?api-version=7.1",
		"https://dev.azure.com/x/proj/_apis/wit/wiql?api-version=7.1",
		"https://dev.azure.com/x/proj/_apis/wit/wiql/abc-guid?api-version=7.1",
		"https://dev.azure.com/x/proj/_apis/wit/queries/abc-guid?api-version=7.1",
	} {
		if err := readOnlyAllowed(http.MethodPost, mustURL(t, raw)); err != nil {
			t.Fatalf("expected POST %s allowed, got %v", raw, err)
		}
	}
}

func TestReadOnlyAllowed_POSTRejected(t *testing.T) {
	for _, raw := range []string{
		"https://dev.azure.com/x/_apis/projects",
		"https://dev.azure.com/x/_apis/git/repositories/r/pullrequests",
		"https://example.com/anything",
	} {
		err := readOnlyAllowed(http.MethodPost, mustURL(t, raw))
		if err == nil {
			t.Fatalf("expected POST %s rejected", raw)
		}
		if !errors.Is(err, ErrWriteAttempted) {
			t.Fatalf("expected ErrWriteAttempted for %s, got %v", raw, err)
		}
	}
}

func TestReadOnlyAllowed_NonGetPostRejected(t *testing.T) {
	u := mustURL(t, "https://dev.azure.com/x/_apis/wit/wiql")
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		err := readOnlyAllowed(m, u)
		if err == nil || !errors.Is(err, ErrWriteAttempted) {
			t.Fatalf("expected %s rejected, got %v", m, err)
		}
	}
}
