package cache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type StoredMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConvMeta struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`
	MessageCount int    `json:"message_count"`
}

type ConvStore struct {
	dir    string
	master []byte
	mu     sync.RWMutex
}

type convIndex struct {
	Convs map[string]ConvMeta `json:"conversations"`
}

type convData struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Messages []StoredMessage `json:"messages"`
}

func NewConvStore(s *Store) *ConvStore {
	dir := filepath.Join(s.dir, "conversations")
	return &ConvStore{
		dir:    dir,
		master: s.master,
	}
}

func (cs *ConvStore) Save(name string, msgs []StoredMessage) (string, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if err := os.MkdirAll(cs.dir, 0700); err != nil {
		return "", fmt.Errorf("conv: cannot create dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	hash := sha256.Sum256([]byte(name + now + fmt.Sprintf("%d", len(msgs))))
	id := base64.RawURLEncoding.EncodeToString(hash[:8])

	data := convData{
		ID:       id,
		Name:     name,
		Messages: msgs,
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("conv: marshal: %w", err)
	}

	enc, err := cs.encrypt(raw)
	if err != nil {
		return "", fmt.Errorf("conv: encrypt: %w", err)
	}

	path := filepath.Join(cs.dir, id+".dat")
	if err := os.WriteFile(path, enc, 0600); err != nil {
		return "", fmt.Errorf("conv: write: %w", err)
	}

	idx, err := cs.loadIndex()
	if err != nil {
		idx = &convIndex{Convs: make(map[string]ConvMeta)}
	}

	idx.Convs[id] = ConvMeta{
		ID:           id,
		Name:         name,
		Created:      now,
		Updated:      now,
		MessageCount: len(msgs),
	}

	if err := cs.saveIndex(idx); err != nil {
		return "", err
	}

	return id, nil
}

func (cs *ConvStore) Load(id string) ([]StoredMessage, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	path := filepath.Join(cs.dir, id+".dat")
	enc, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("conv: not found: %w", err)
	}

	raw, err := cs.decrypt(enc)
	if err != nil {
		return nil, fmt.Errorf("conv: decrypt: %w", err)
	}

	var data convData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("conv: corrupt: %w", err)
	}

	return data.Messages, nil
}

func (cs *ConvStore) List() ([]ConvMeta, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	idx, err := cs.loadIndex()
	if err != nil {
		return nil, nil
	}

	out := make([]ConvMeta, 0, len(idx.Convs))
	for _, m := range idx.Convs {
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Updated > out[j].Updated
	})

	return out, nil
}

func (cs *ConvStore) Delete(id string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	path := filepath.Join(cs.dir, id+".dat")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("conv: not found")
		}
		return fmt.Errorf("conv: delete: %w", err)
	}

	idx, err := cs.loadIndex()
	if err == nil && idx.Convs != nil {
		delete(idx.Convs, id)
		_ = cs.saveIndex(idx)
	}

	return nil
}

func (cs *ConvStore) UpdateMeta(id string, msgs []StoredMessage) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	idx, err := cs.loadIndex()
	if err != nil {
		return err
	}

	m, ok := idx.Convs[id]
	if !ok {
		return fmt.Errorf("conv: not found")
	}

	m.MessageCount = len(msgs)
	m.Updated = time.Now().UTC().Format(time.RFC3339)
	idx.Convs[id] = m

	return cs.saveIndex(idx)
}

func (cs *ConvStore) indexPath() string {
	return filepath.Join(cs.dir, "index.json")
}

func (cs *ConvStore) loadIndex() (*convIndex, error) {
	data, err := os.ReadFile(cs.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &convIndex{Convs: make(map[string]ConvMeta)}, nil
		}
		return nil, fmt.Errorf("conv: read index: %w", err)
	}

	var idx convIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("conv: corrupt index: %w", err)
	}

	if idx.Convs == nil {
		idx.Convs = make(map[string]ConvMeta)
	}

	return &idx, nil
}

func (cs *ConvStore) saveIndex(idx *convIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("conv: marshal index: %w", err)
	}

	tmp := cs.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("conv: write index: %w", err)
	}

	return os.Rename(tmp, cs.indexPath())
}

func (cs *ConvStore) encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(cs.master)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plain, nil)
	out := make([]byte, len(nonce)+len(ciphertext))
	copy(out, nonce)
	copy(out[len(nonce):], ciphertext)
	return out, nil
}

func (cs *ConvStore) decrypt(data []byte) ([]byte, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("conv: too short")
	}

	nonce := data[:12]
	ciphertext := data[12:]

	block, err := aes.NewCipher(cs.master)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aesgcm.Open(nil, nonce, ciphertext, nil)
}
