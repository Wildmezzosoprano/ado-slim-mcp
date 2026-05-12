//go:build !windows

package auth

import "errors"

// Non-Windows error stubs. Kept purely for symmetry — the storage layer on
// non-Windows uses identity functions in cache_storage_other.go and never
// invokes these.

func protect(_, _ []byte) ([]byte, error) {
	return nil, errors.New("dpapi: unsupported on non-windows")
}

func unprotect(_, _ []byte) ([]byte, error) {
	return nil, errors.New("dpapi: unsupported on non-windows")
}
