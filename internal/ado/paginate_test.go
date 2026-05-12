package ado

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPaginate_EmptyAndNilInputsReturnNonNilItems(t *testing.T) {
	cases := []struct {
		name  string
		input []int
		top   int
		skip  int
	}{
		{name: "nil input", input: nil, top: 50, skip: 0},
		{name: "empty non-nil input", input: []int{}, top: 50, skip: 0},
		{name: "skip past end", input: []int{1, 2, 3}, top: 50, skip: 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := Paginate[int](tc.input, tc.top, tc.skip)
			if resp.Items == nil {
				t.Fatalf("Items should be non-nil, got nil")
			}
			if len(resp.Items) != 0 {
				t.Fatalf("Items should be empty, got len=%d", len(resp.Items))
			}
			b, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			s := string(b)
			if !strings.Contains(s, `"items":[]`) {
				t.Fatalf("expected JSON to contain `\"items\":[]`, got: %s", s)
			}
			if strings.Contains(s, `"items":null`) {
				t.Fatalf("expected JSON to NOT contain `\"items\":null`, got: %s", s)
			}
		})
	}
}

func TestPaginate_HappyPath(t *testing.T) {
	resp := Paginate[int]([]int{1, 2, 3}, 2, 0)
	if resp.Count != 2 {
		t.Fatalf("Count: want 2, got %d", resp.Count)
	}
	if !resp.HasMore {
		t.Fatalf("HasMore: want true, got false")
	}
	if len(resp.Items) != 2 || resp.Items[0] != 1 || resp.Items[1] != 2 {
		t.Fatalf("Items: want [1 2], got %v", resp.Items)
	}
}
