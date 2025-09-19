package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type Authenticator struct {
	keys           map[string]string // appid -> key
	timeWindowSec  int64
	nonceStore     NonceStore
}

type NonceStore interface {
	HasNonce(nonce string) bool
	AddNonce(nonce string, ttl time.Duration) error
}

func NewAuthenticator(keys map[string]string, timeWindowSec int64, nonceStore NonceStore) *Authenticator {
	return &Authenticator{
		keys:          keys,
		timeWindowSec: timeWindowSec,
		nonceStore:    nonceStore,
	}
}

func (a *Authenticator) VerifySignature(appid, sn string, ts int64, nonce, signature string) error {
	key, exists := a.keys[appid]
	if !exists {
		return fmt.Errorf("invalid appid")
	}

	if err := a.verifyTimestamp(ts); err != nil {
		return err
	}

	if err := a.verifyNonce(nonce); err != nil {
		return err
	}

	expectedSig := a.calculateSignature(appid, sn, ts, nonce, key)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func (a *Authenticator) calculateSignature(appid, sn string, ts int64, nonce, key string) string {
	payload := fmt.Sprintf("%s|%s|%d|%s", appid, sn, ts, nonce)
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func (a *Authenticator) verifyTimestamp(ts int64) error {
	now := time.Now().Unix()
	if abs(now-ts) > a.timeWindowSec {
		return fmt.Errorf("timestamp out of window")
	}
	return nil
}

func (a *Authenticator) verifyNonce(nonce string) error {
	if a.nonceStore.HasNonce(nonce) {
		return fmt.Errorf("nonce already used")
	}

	ttl := time.Duration(a.timeWindowSec*2) * time.Second
	if err := a.nonceStore.AddNonce(nonce, ttl); err != nil {
		return fmt.Errorf("failed to store nonce: %w", err)
	}

	return nil
}

func (a *Authenticator) GenerateSignature(appid, sn string, ts int64, nonce, key string) string {
	return a.calculateSignature(appid, sn, ts, nonce, key)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

type MemoryNonceStore struct {
	nonces map[string]time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{
		nonces: make(map[string]time.Time),
	}
}

func (m *MemoryNonceStore) HasNonce(nonce string) bool {
	_, exists := m.nonces[nonce]
	return exists
}

func (m *MemoryNonceStore) AddNonce(nonce string, ttl time.Duration) error {
	m.nonces[nonce] = time.Now().Add(ttl)
	return nil
}

func (m *MemoryNonceStore) Cleanup() {
	now := time.Now()
	for nonce, expiry := range m.nonces {
		if now.After(expiry) {
			delete(m.nonces, nonce)
		}
	}
}