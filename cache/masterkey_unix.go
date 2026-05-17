//go:build !windows

package cache

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) protectMasterKey(plain []byte) ([]byte, error) {
	enc := make([]byte, base64.RawStdEncoding.EncodedLen(len(plain)))
	base64.RawStdEncoding.Encode(enc, plain)
	return enc, nil
}

func (s *Store) unprotectMasterKey(cipher []byte) error {
	plain := make([]byte, base64.RawStdEncoding.DecodedLen(len(cipher)))
	n, err := base64.RawStdEncoding.Decode(plain, cipher)
	if err != nil {
		return fmt.Errorf("cache: invalid master key encoding: %w", err)
	}
	if n != 32 {
		return fmt.Errorf("cache: invalid master key length %d", n)
	}
	s.master = plain[:n]
	return nil
}

func ensureKeyPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}

	if info.Mode().Perm() != 0600 {
		return os.Chmod(path, 0600)
	}
	return nil
}
