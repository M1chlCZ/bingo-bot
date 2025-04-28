package security

import (
	"github.com/M1chlCZ/bingo-bot/audit"
	"github.com/M1chlCZ/bingo-bot/errors"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/adshao/go-binance/v2"
)

const (
	// KeyIDs for the keystore
	BinanceAPIKeyID     = "binance_api_key"
	BinanceAPISecretID  = "binance_api_secret"
)

// SecureBinanceClientProvider provides a secure way to create and access a Binance client
type SecureBinanceClientProvider struct {
	keyStore *KeyStore
	client   *binance.Client
}

// NewSecureBinanceClientProvider creates a new SecureBinanceClientProvider
func NewSecureBinanceClientProvider(keyStore *KeyStore) *SecureBinanceClientProvider {
	return &SecureBinanceClientProvider{
		keyStore: keyStore,
	}
}

// StoreCredentials securely stores the Binance API credentials
func (p *SecureBinanceClientProvider) StoreCredentials(apiKey, apiSecret string) error {
	// Audit log the attempt
	audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", true, "Attempting to store Binance API credentials", nil)

	// Store API key
	if err := p.keyStore.StoreKey(BinanceAPIKeyID, apiKey); err != nil {
		// Audit log the failure
		audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", false, "Failed to store API key", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Store API secret
	if err := p.keyStore.StoreKey(BinanceAPISecretID, apiSecret); err != nil {
		// Audit log the failure
		audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", false, "Failed to store API secret", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Audit log the success
	audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", true, "Successfully stored Binance API credentials", nil)

	logger.Debugf("Binance API credentials stored securely")
	return nil
}

// GetClient returns a Binance client with the stored credentials
func (p *SecureBinanceClientProvider) GetClient() (*binance.Client, error) {
	// Audit log the attempt
	audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", true, "Attempting to get Binance client", nil)

	// If client already exists, return it
	if p.client != nil {
		// Audit log the success (cached client)
		audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", true, "Retrieved cached Binance client", nil)
		return p.client, nil
	}

	// Retrieve API key
	apiKey, err := p.keyStore.GetKey(BinanceAPIKeyID)
	if err != nil {
		// Audit log the failure
		audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", false, "Failed to retrieve API key", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Retrieve API secret
	apiSecret, err := p.keyStore.GetKey(BinanceAPISecretID)
	if err != nil {
		// Audit log the failure
		audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", false, "Failed to retrieve API secret", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Create new client
	p.client = binance.NewClient(apiKey, apiSecret)

	// Audit log the success
	audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", true, "Successfully created new Binance client", nil)

	return p.client, nil
}

// Global SecureBinanceClientProvider instance
var globalBinanceProvider *SecureBinanceClientProvider

// InitSecureBinanceProvider initializes the global SecureBinanceClientProvider
func InitSecureBinanceProvider(apiKey, apiSecret string) error {
	// Audit log the attempt
	audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", true, "Attempting to initialize global Binance provider", nil)

	// Initialize global keystore if not already initialized
	if err := InitGlobalKeyStore(); err != nil {
		// Audit log the failure
		audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", false, "Failed to initialize global keystore", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Get global keystore
	keyStore, err := GetGlobalKeyStore()
	if err != nil {
		// Audit log the failure
		audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", false, "Failed to get global keystore", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Create provider
	globalBinanceProvider = NewSecureBinanceClientProvider(keyStore)

	// Store credentials
	err = globalBinanceProvider.StoreCredentials(apiKey, apiSecret)
	if err != nil {
		// Audit log the failure
		audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", false, "Failed to store credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	// Audit log the success
	audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", true, "Successfully initialized global Binance provider", nil)

	return nil
}

// GetSecureBinanceClient returns a Binance client from the global provider
func GetSecureBinanceClient() (*binance.Client, error) {
	// Audit log the attempt
	audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", true, "Attempting to get secure Binance client", nil)

	if globalBinanceProvider == nil {
		// Audit log the failure
		audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", false, "Global Binance provider not initialized", nil)
		return nil, errors.NewWithType(errors.ErrConfigurationError, "global Binance provider not initialized")
	}

	client, err := globalBinanceProvider.GetClient()
	if err != nil {
		// Audit log the failure
		audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", false, "Failed to get client from provider", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	// Audit log the success
	audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", true, "Successfully retrieved secure Binance client", nil)

	return client, nil
}
