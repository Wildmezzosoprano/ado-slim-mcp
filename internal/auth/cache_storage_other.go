//go:build !windows

package auth

// Identity helpers for non-Windows platforms. The MSAL token cache is written
// as plaintext JSON with mode 0600 (preserved in cache.go). wasEncrypted is
// always false. Preserves byte-for-byte on-disk compatibility with the
// TypeScript implementation.

func encryptForStorage(plain []byte) ([]byte, error) {
	return plain, nil
}

func decryptFromStorage(stored []byte) ([]byte, bool, error) {
	return stored, false, nil
}
