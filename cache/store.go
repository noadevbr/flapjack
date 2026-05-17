package cache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const appDir = ".flapjack"
const storeFile = "store.dat"
const keyFile = "master.key.enc"

type encryptedValue struct {
	Nonce   string `json:"nonce"`
	Payload string `json:"payload"`
}

type secureStore struct {
	Values map[string]encryptedValue `json:"values"`
}

type Store struct {
	dir      string
	master   []byte
	values   map[string]encryptedValue
	mu       sync.RWMutex
	loaded   bool
}

func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cache: cannot find home dir: %w", err)
	}
	return filepath.Join(home, appDir), nil
}

func Open() (*Store, error) {
	dir, err := DefaultDir()
	if err != nil {
		return nil, err
	}
	return OpenDir(dir)
}

func OpenDir(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("cache: cannot create dir %s: %w", dir, err)
	}

	s := &Store{dir: dir}

	if err := s.loadMasterKey(); err != nil {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("cache: failed to generate master key: %w", err)
		}
		s.master = key
		if err := s.saveMasterKey(); err != nil {
			return nil, fmt.Errorf("cache: failed to save master key: %w", err)
		}
	}

	return s, nil
}

func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return err
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("cache: failed to generate nonce: %w", err)
	}

	block, err := aes.NewCipher(s.master)
	if err != nil {
		return fmt.Errorf("cache: aes init: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("cache: gcm init: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(value), nil)

	s.values[key] = encryptedValue{
		Nonce:   base64.RawStdEncoding.EncodeToString(nonce),
		Payload: base64.RawStdEncoding.EncodeToString(ciphertext),
	}

	return s.flush()
}

func (s *Store) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return "", err
	}

	ev, ok := s.values[key]
	if !ok {
		return "", fmt.Errorf("cache: key %q not found", key)
	}

	nonce, err := base64.RawStdEncoding.DecodeString(ev.Nonce)
	if err != nil {
		return "", fmt.Errorf("cache: invalid nonce for key %q: %w", key, err)
	}

	ciphertext, err := base64.RawStdEncoding.DecodeString(ev.Payload)
	if err != nil {
		return "", fmt.Errorf("cache: invalid payload for key %q: %w", key, err)
	}

	block, err := aes.NewCipher(s.master)
	if err != nil {
		return "", fmt.Errorf("cache: aes init: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cache: gcm init: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("cache: decryption failed (tampered or wrong key) for %q: %w", key, err)
	}

	return string(plaintext), nil
}

func (s *Store) List() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(s.values))
	for k := range s.values {
		out[k] = "(set)"
	}
	return out, nil
}

func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return err
	}

	delete(s.values, key)
	return s.flush()
}

func (s *Store) storePath() string {
	return filepath.Join(s.dir, storeFile)
}

func (s *Store) load() error {
	if s.loaded {
		return nil
	}

	data, err := os.ReadFile(s.storePath())
	if err != nil {
		if os.IsNotExist(err) {
			s.values = make(map[string]encryptedValue)
			s.loaded = true
			return nil
		}
		return fmt.Errorf("cache: cannot read store: %w", err)
	}

	var st secureStore
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("cache: corrupted store file: %w", err)
	}

	if st.Values == nil {
		st.Values = make(map[string]encryptedValue)
	}

	s.values = st.Values
	s.loaded = true
	return nil
}

func (s *Store) flush() error {
	st := secureStore{Values: s.values}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("cache: cannot marshal store: %w", err)
	}

	tmpPath := s.storePath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("cache: cannot write store: %w", err)
	}

	if err := os.Rename(tmpPath, s.storePath()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("cache: cannot rename store: %w", err)
	}

	return nil
}

func (s *Store) loadMasterKey() error {
	path := filepath.Join(s.dir, keyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return s.unprotectMasterKey(data)
}

func (s *Store) saveMasterKey() error {
	protected, err := s.protectMasterKey(s.master)
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, keyFile)
	return os.WriteFile(path, protected, 0600)
}
