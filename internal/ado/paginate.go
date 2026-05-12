// Package ado contains the Azure DevOps HTTP client wrapper, URL helpers,
// and pagination utilities. Mirrors `ado-slim-mcp/src/pagination.ts`.
package ado

const (
	// DefaultTop is the default page size when callers omit `top`.
	DefaultTop = 50
	// MaxBatchSize caps work-item batch fetches per ADO REST limits.
	MaxBatchSize = 200
)

// ListResponse is the universal list envelope returned by every list-shaped
// tool. Field tags match the TS interface in `src/types.ts` (camelCase).
type ListResponse[T any] struct {
	Count   int  `json:"count"`
	HasMore bool `json:"hasMore"`
	Items   []T  `json:"items"`
}

// Paginate slices the full result set into a page bounded by [skip, skip+top)
// and reports whether more items exist beyond the page.
func Paginate[T any](allItems []T, top, skip int) ListResponse[T] {
	if skip < 0 {
		skip = 0
	}
	if top < 0 {
		top = 0
	}
	end := skip + top
	if end > len(allItems) {
		end = len(allItems)
	}
	if skip > len(allItems) {
		skip = len(allItems)
	}
	sliced := allItems[skip:end]
	if sliced == nil {
		sliced = make([]T, 0)
	}
	return ListResponse[T]{
		Count:   len(sliced),
		HasMore: skip+top < len(allItems),
		Items:   sliced,
	}
}

// BatchIDs splits a slice of IDs into batches of MaxBatchSize.
func BatchIDs(ids []int) [][]int {
	if len(ids) == 0 {
		return nil
	}
	batches := make([][]int, 0, (len(ids)+MaxBatchSize-1)/MaxBatchSize)
	for i := 0; i < len(ids); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		batches = append(batches, ids[i:end])
	}
	return batches
}

// ResolveTopSkip applies defaults (50/0) when callers pass nil pointers,
// matching the TS `resolveTopSkip(top?, skip?)` semantics.
func ResolveTopSkip(top, skip *int) (int, int) {
	t := DefaultTop
	if top != nil {
		t = *top
	}
	s := 0
	if skip != nil {
		s = *skip
	}
	return t, s
}
