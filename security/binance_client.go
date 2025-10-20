package security

import (
	"github.com/M1chlCZ/bingo-bot/audit"
	"github.com/M1chlCZ/bingo-bot/errors"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/adshao/go-binance/v2"
)

const (
	BinanceAPIKeyID    = "binance_api_key"
	BinanceAPISecretID = "binance_api_secret"
)

type SecureBinanceClientProvider struct {
	keyStore *KeyStore
	client   *binance.Client
}

func NewSecureBinanceClientProvider(keyStore *KeyStore) *SecureBinanceClientProvider {
	return &SecureBinanceClientProvider{
		keyStore: keyStore,
	}
}

func (p *SecureBinanceClientProvider) StoreCredentials(apiKey, apiSecret string) error {

	audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", true, "Attempting to store Binance API credentials", nil)

	if err := p.keyStore.StoreKey(BinanceAPIKeyID, apiKey); err != nil {

		audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", false, "Failed to store API key", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	if err := p.keyStore.StoreKey(BinanceAPISecretID, apiSecret); err != nil {

		audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", false, "Failed to store API secret", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	audit.LogCreate("security.SecureBinanceClientProvider", "StoreCredentials", "BinanceAPICredentials", true, "Successfully stored Binance API credentials", nil)

	logger.Debugf("Binance API credentials stored securely")
	return nil
}

func (p *SecureBinanceClientProvider) GetClient() (*binance.Client, error) {

	audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", true, "Attempting to get Binance client", nil)

	if p.client != nil {

		audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", true, "Retrieved cached Binance client", nil)
		return p.client, nil
	}

	apiKey, err := p.keyStore.GetKey(BinanceAPIKeyID)
	if err != nil {

		audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", false, "Failed to retrieve API key", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	apiSecret, err := p.keyStore.GetKey(BinanceAPISecretID)
	if err != nil {

		audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", false, "Failed to retrieve API secret", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	p.client = binance.NewClient(apiKey, apiSecret)

	audit.LogAccess("security.SecureBinanceClientProvider", "GetClient", "BinanceClient", true, "Successfully created new Binance client", nil)

	return p.client, nil
}

var globalBinanceProvider *SecureBinanceClientProvider

func InitSecureBinanceProvider(apiKey, apiSecret string) error {

	audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", true, "Attempting to initialize global Binance provider", nil)

	if err := InitGlobalKeyStore(); err != nil {

		audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", false, "Failed to initialize global keystore", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	keyStore, err := GetGlobalKeyStore()
	if err != nil {

		audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", false, "Failed to get global keystore", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	globalBinanceProvider = NewSecureBinanceClientProvider(keyStore)

	err = globalBinanceProvider.StoreCredentials(apiKey, apiSecret)
	if err != nil {

		audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", false, "Failed to store credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	audit.LogCreate("security", "InitSecureBinanceProvider", "GlobalBinanceProvider", true, "Successfully initialized global Binance provider", nil)

	return nil
}

func GetSecureBinanceClient() (*binance.Client, error) {

	audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", true, "Attempting to get secure Binance client", nil)

	if globalBinanceProvider == nil {

		audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", false, "Global Binance provider not initialized", nil)
		return nil, errors.NewWithType(errors.ErrConfigurationError, "global Binance provider not initialized")
	}

	client, err := globalBinanceProvider.GetClient()
	if err != nil {

		audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", false, "Failed to get client from provider", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	audit.LogAccess("security", "GetSecureBinanceClient", "GlobalBinanceClient", true, "Successfully retrieved secure Binance client", nil)

	return client, nil
}
