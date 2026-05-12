//go:build windows

package auth

import (
	"bytes"
	"testing"
)

func TestProtectUnprotectRoundTrip(t *testing.T) {
	plain := []byte("hello world")
	entropy := []byte("entropy-1")
	cipher, err := protect(plain, entropy)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	if bytes.Equal(cipher, plain) {
		t.Fatalf("ciphertext equals plaintext — DPAPI did nothing")
	}
	got, err := unprotect(cipher, entropy)
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plain)
	}
}

func TestUnprotectWrongEntropyFails(t *testing.T) {
	plain := []byte("hello world")
	cipher, err := protect(plain, []byte("entropy-A"))
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	if _, err := unprotect(cipher, []byte("entropy-B")); err == nil {
		t.Fatalf("expected error from unprotect with wrong entropy, got nil")
	}
}

func TestProtectUnprotectEmptyEntropy(t *testing.T) {
	plain := []byte("hello")
	cipher, err := protect(plain, nil)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	got, err := unprotect(cipher, nil)
	if err != nil {
		t.Fatalf("unprotect: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("empty-entropy round-trip mismatch: got %q want %q", got, plain)
	}
}

func TestProtectUnprotectEmptyPlaintext(t *testing.T) {
	cipher, err := protect([]byte{}, []byte("e"))
	if err != nil {
		// Document actual behavior: some DPAPI builds reject zero-length input.
		t.Logf("protect of empty plaintext failed (acceptable): %v", err)
		return
	}
	got, err := unprotect(cipher, []byte("e"))
	if err != nil {
		t.Fatalf("unprotect of empty round-trip: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty plaintext, got %q", got)
	}
}

func TestDecryptFromStorageWasEncryptedFlag(t *testing.T) {
	// Magic + valid ciphertext → wasEncrypted=true, no error.
	cipher, err := protect([]byte(`{"AccessToken":{}}`), storageEntropy)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	buf := append([]byte(storageMagic), cipher...)
	plain, was, err := decryptFromStorage(buf)
	if err != nil {
		t.Fatalf("decryptFromStorage(valid): %v", err)
	}
	if !was {
		t.Fatalf("wasEncrypted=false for magic-prefixed input")
	}
	if !bytes.Equal(plain, []byte(`{"AccessToken":{}}`)) {
		t.Fatalf("decrypted bytes mismatch: %q", plain)
	}

	// No magic → wasEncrypted=false, no error, bytes returned verbatim.
	legacy := []byte(`{"AccessToken":{}}`)
	plain, was, err = decryptFromStorage(legacy)
	if err != nil {
		t.Fatalf("decryptFromStorage(legacy): %v", err)
	}
	if was {
		t.Fatalf("wasEncrypted=true for legacy plaintext input")
	}
	if !bytes.Equal(plain, legacy) {
		t.Fatalf("legacy bytes mismatch")
	}

	// Magic + garbage → wasEncrypted=true, error.
	garbage := append([]byte(storageMagic), []byte("not real ciphertext")...)
	_, was, err = decryptFromStorage(garbage)
	if err == nil {
		t.Fatalf("expected error from decryptFromStorage(garbage), got nil")
	}
	if !was {
		t.Fatalf("wasEncrypted=false for magic-prefixed garbage")
	}
}
