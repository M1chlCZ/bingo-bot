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

type KeyStore struct {
	encryptedKeys map[string]string
	masterKey     []byte
	mu            sync.RWMutex
}

func NewKeyStore() (*KeyStore, error) {

	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		return nil, err
	}

	return &KeyStore{
		encryptedKeys: make(map[string]string),
		masterKey:     masterKey,
	}, nil
}

func (ks *KeyStore) StoreKey(id, key string) error {
	if id == "" || key == "" {

		audit.LogCreate("security.KeyStore", "StoreKey", id, false, "Failed to store key: empty id or key", nil)
		return errors.New("id and key cannot be empty")
	}

	encryptedKey, err := encrypt(key, ks.masterKey)
	if err != nil {

		audit.LogCreate("security.KeyStore", "StoreKey", id, false, "Failed to encrypt key", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	ks.mu.Lock()
	ks.encryptedKeys[id] = encryptedKey
	ks.mu.Unlock()

	audit.LogCreate("security.KeyStore", "StoreKey", id, true, "Successfully stored encrypted key", nil)

	logger.Debugf("Stored key with ID: %s", id)
	return nil
}

func (ks *KeyStore) GetKey(id string) (string, error) {
	ks.mu.RLock()
	encryptedKey, exists := ks.encryptedKeys[id]
	ks.mu.RUnlock()

	if !exists {

		audit.LogAccess("security.KeyStore", "GetKey", id, false, "Key not found", nil)
		return "", errors.New("key not found")
	}

	key, err := decrypt(encryptedKey, ks.masterKey)
	if err != nil {

		audit.LogAccess("security.KeyStore", "GetKey", id, false, "Failed to decrypt key", map[string]interface{}{
			"error": err.Error(),
		})
		return "", err
	}

	audit.LogAccess("security.KeyStore", "GetKey", id, true, "Successfully retrieved key", nil)

	return key, nil
}

func encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	return base64.StdEncoding.EncodeToString(result), nil
}

func decrypt(encryptedText string, key []byte) (string, error) {

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < 12 {
		return "", errors.New("ciphertext too short")
	}

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

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

var (
	globalKeyStore *KeyStore
	initOnce       sync.Once
	initErr        error
)

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

func GetGlobalKeyStore() (*KeyStore, error) {
	if globalKeyStore == nil {
		return nil, errors.New("global KeyStore not initialized")
	}
	return globalKeyStore, nil
}
