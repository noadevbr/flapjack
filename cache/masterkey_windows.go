//go:build windows

package cache

import (
	"encoding/base64"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	crypt32              = syscall.NewLazyDLL("crypt32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

const cryptProtectUIForbidden = 0x01

func (s *Store) protectMasterKey(plain []byte) ([]byte, error) {
	in := dataBlob{
		cbData: uint32(len(plain)),
		pbData: &plain[0],
	}

	var out dataBlob
	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("dpapi: CryptProtectData failed: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	enc := unsafe.Slice(out.pbData, out.cbData)
	buf := make([]byte, base64.RawStdEncoding.EncodedLen(len(enc)))
	base64.RawStdEncoding.Encode(buf, enc)
	return buf, nil
}

func (s *Store) unprotectMasterKey(cipher []byte) error {
	dec := make([]byte, base64.RawStdEncoding.DecodedLen(len(cipher)))
	n, err := base64.RawStdEncoding.Decode(dec, cipher)
	if err != nil {
		return fmt.Errorf("dpapi: base64 decode failed: %w", err)
	}
	dec = dec[:n]

	in := dataBlob{cbData: uint32(len(dec)), pbData: &dec[0]}
	var out dataBlob

	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return fmt.Errorf("dpapi: CryptUnprotectData failed: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	key := unsafe.Slice(out.pbData, out.cbData)
	if len(key) != 32 {
		return fmt.Errorf("dpapi: invalid master key length %d", len(key))
	}

	s.master = make([]byte, 32)
	copy(s.master, key)
	return nil
}
