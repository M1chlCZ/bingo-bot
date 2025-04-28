package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"sync"

	"github.com/M1chlCZ/bingo-bot/audit"
	"github.com/M1chlCZ/bingo-bot/logger"
)

// KeyStore provides secure storage for sensitive API keys and secrets
type KeyStore struct {
	encryptedKeys map[string]string
	masterKey     []byte
	mu            sync.RWMutex
}

// NewKeyStore creates a new KeyStore with a randomly generated master key
func NewKeyStore() (*KeyStore, error) {
	// Generate a random 32-byte key for AES-256
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		return nil, err
	}

	return &KeyStore{
		encryptedKeys: make(map[string]string),
		masterKey:     masterKey,
	}, nil
}

// StoreKey securely stores a key with the given identifier
func (ks *KeyStore) StoreKey(id, key string) error {
	if id == "" || key == "" {
		// Audit log the failure
		audit.LogCreate("security.KeyStore", "StoreKey", id, false, "Failed to store key: empty id or key", nil)
		return errors.New("id and key cannot be empty")
	}

	// Encrypt the key
	encryptedKey, err := encrypt(key, ks.masterKey)
	if err != nil {
		// Audit log the failure
		audit.LogCreate("security.KeyStore", "StoreKey", id, false, "Failed to encrypt key", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Store the encrypted key
	ks.mu.Lock()
	ks.encryptedKeys[id] = encryptedKey
	ks.mu.Unlock()

	// Audit log the success
	audit.LogCreate("security.KeyStore", "StoreKey", id, true, "Successfully stored encrypted key", nil)

	logger.Debugf("Stored key with ID: %s", id)
	return nil
}

// GetKey retrieves a key by its identifier
func (ks *KeyStore) GetKey(id string) (string, error) {
	ks.mu.RLock()
	encryptedKey, exists := ks.encryptedKeys[id]
	ks.mu.RUnlock()

	if !exists {
		// Audit log the failure
		audit.LogAccess("security.KeyStore", "GetKey", id, false, "Key not found", nil)
		return "", errors.New("key not found")
	}

	// Decrypt the key
	key, err := decrypt(encryptedKey, ks.masterKey)
	if err != nil {
		// Audit log the failure
		audit.LogAccess("security.KeyStore", "GetKey", id, false, "Failed to decrypt key", map[string]interface{}{
			"error": err.Error(),
		})
		return "", err
	}

	// Audit log the success
	audit.LogAccess("security.KeyStore", "GetKey", id, true, "Successfully retrieved key", nil)

	return key, nil
}

// encrypt encrypts plaintext using AES-GCM with the provided key
func encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Never use more than 2^32 random nonces with a given key
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Encrypt and authenticate
	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	// Base64 encode for storage
	return base64.StdEncoding.EncodeToString(result), nil
}

// decrypt decrypts ciphertext using AES-GCM with the provided key
func decrypt(encryptedText string, key []byte) (string, error) {
	// Base64 decode
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	// Check if the ciphertext is long enough to contain a nonce
	if len(ciphertext) < 12 {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce from the beginning of the ciphertext
	nonce := ciphertext[:12]
	ciphertext = ciphertext[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Decrypt and verify
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Global KeyStore instance
var (
	globalKeyStore *KeyStore
	initOnce       sync.Once
	initErr        error
)

// InitGlobalKeyStore initializes the global KeyStore
func InitGlobalKeyStore() error {
	initOnce.Do(func() {
		var ks *KeyStore
		ks, initErr = NewKeyStore()
		if initErr != nil {
			return
		}
		globalKeyStore = ks
	})
	return initErr
}

// GetGlobalKeyStore returns the global KeyStore instance
func GetGlobalKeyStore() (*KeyStore, error) {
	if globalKeyStore == nil {
		return nil, errors.New("global KeyStore not initialized")
	}
	return globalKeyStore, nil
}
