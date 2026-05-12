//go:build windows

package auth

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPI primitives (CryptProtectData / CryptUnprotectData) with CurrentUser
// scope and caller-supplied entropy. Pure primitives — no framing, no magic
// prefix; that lives in cache_storage_windows.go.
//
// Buffer-lifetime note: the syscall reads from Go-owned memory via raw
// pointers stuffed inside dataBlob.Data. The Go GC cannot see those, so we
// MUST issue an explicit runtime.KeepAlive(...) for each input slice AFTER
// the syscall returns. See plan step 1 + risk register.

// dataBlob mirrors Win32 DATA_BLOB { DWORD cbData; BYTE *pbData; }.
type dataBlob struct {
	Size uint32
	Data *byte
}

func newBlob(buf []byte) dataBlob {
	if len(buf) == 0 {
		return dataBlob{Size: 0, Data: nil}
	}
	return dataBlob{Size: uint32(len(buf)), Data: &buf[0]}
}

var (
	modCrypt32           = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData = modCrypt32.NewProc("CryptProtectData")
	procCryptUnprotect   = modCrypt32.NewProc("CryptUnprotectData")
)

const cryptProtectUIForbidden = 0x1

// protect encrypts plaintext under DPAPI CurrentUser scope, bound by the
// caller-supplied entropy (may be empty). Returns a Go-owned []byte copy of
// the ciphertext; the OS-allocated buffer is freed before return.
func protect(plaintext, entropy []byte) ([]byte, error) {
	in := newBlob(plaintext)
	ent := newBlob(entropy)
	var out dataBlob

	var entPtr *dataBlob
	if len(entropy) > 0 {
		entPtr = &ent
	}

	ret, _, callErr := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // szDataDescr
		uintptr(unsafe.Pointer(entPtr)),
		0, // pvReserved
		0, // pPromptStruct
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		// callErr is set from GetLastError by syscall machinery
		return nil, fmt.Errorf("CryptProtectData failed: %w", callErr)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))

	var cipher []byte
	if out.Size > 0 && out.Data != nil {
		raw := unsafe.Slice(out.Data, int(out.Size))
		cipher = append([]byte(nil), raw...)
	} else {
		cipher = []byte{}
	}

	runtime.KeepAlive(plaintext)
	runtime.KeepAlive(entropy)
	return cipher, nil
}

// unprotect decrypts ciphertext previously produced by protect with the same
// entropy. Returns a Go-owned []byte copy of the plaintext.
func unprotect(ciphertext, entropy []byte) ([]byte, error) {
	in := newBlob(ciphertext)
	ent := newBlob(entropy)
	var out dataBlob

	var entPtr *dataBlob
	if len(entropy) > 0 {
		entPtr = &ent
	}

	ret, _, callErr := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, // ppszDataDescr
		uintptr(unsafe.Pointer(entPtr)),
		0, // pvReserved
		0, // pPromptStruct
		uintptr(cryptProtectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %w", callErr)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))

	var plain []byte
	if out.Size > 0 && out.Data != nil {
		raw := unsafe.Slice(out.Data, int(out.Size))
		plain = append([]byte(nil), raw...)
	} else {
		plain = []byte{}
	}

	runtime.KeepAlive(ciphertext)
	runtime.KeepAlive(entropy)
	return plain, nil
}
