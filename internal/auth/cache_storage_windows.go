//go:build windows

package auth

import "bytes"

// On-disk format for the MSAL token cache on Windows:
//
//   [10-byte magic "ADOSLIMV1\n"] [DPAPI ciphertext...]
//
// The magic prefix is a self-versioning format discriminator: if a future
// migration is needed (LocalMachine scope, different KDF, etc.), bump to
// "ADOSLIMV2\n" + new entropy and add a V2 decrypt branch.

const storageMagic = "ADOSLIMV1\n"

// storageEntropy app-binds the DPAPI ciphertext: an unrelated process running
// as the same Windows user that calls CryptUnprotectData on the raw blob
// without this entropy will fail.
//
// WARNING — CHANGING THIS VALUE INVALIDATES EVERY EXISTING CACHE. THIS IS A
// PERMANENT CONTRACT FOR THE V1 FORMAT. IF YOU NEED TO CHANGE THE ENTROPY,
// BUMP THE MAGIC PREFIX TO "ADOSLIMV2\n" AND ADD A V2 DECRYPT BRANCH IN
// decryptFromStorage. DO NOT EDIT IN PLACE.
var storageEntropy = []byte("ado-slim-mcp/v1")

// encryptForStorage takes plaintext MSAL JSON, DPAPI-encrypts it under
// storageEntropy, and prepends the magic prefix.
func encryptForStorage(plain []byte) ([]byte, error) {
	cipher, err := protect(plain, storageEntropy)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(storageMagic)+len(cipher))
	out = append(out, []byte(storageMagic)...)
	out = append(out, cipher...)
	return out, nil
}

// decryptFromStorage inspects stored bytes:
//
//   - magic-prefixed: strip prefix, DPAPI-decrypt, return (plain, true, nil)
//     on success or (nil, true, err) on DPAPI failure (caller wraps with a
//     user-actionable message).
//   - no prefix: legacy plaintext path, return (stored, false, nil).
func decryptFromStorage(stored []byte) ([]byte, bool, error) {
	prefix := []byte(storageMagic)
	if !bytes.HasPrefix(stored, prefix) {
		return stored, false, nil
	}
	cipher := stored[len(prefix):]
	plain, err := unprotect(cipher, storageEntropy)
	if err != nil {
		return nil, true, err
	}
	return plain, true, nil
}
