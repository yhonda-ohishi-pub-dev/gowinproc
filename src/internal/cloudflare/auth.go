package cloudflare

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/yhonda-ohishi-pub-dev/go_auth/pkg/authclient"
)

// AuthClient handles authentication with Cloudflare Workers
type AuthClient struct {
	client *authclient.Client
}

// NewAuthClient creates a new Cloudflare auth client using go_auth library
func NewAuthClient(workerURL, privateKeyPath, clientID string) (*AuthClient, error) {
	// Load private key
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	// Create go_auth client
	client, err := authclient.NewClient(authclient.ClientConfig{
		BaseURL:    workerURL,
		ClientID:   clientID,
		PrivateKey: privateKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client: %w", err)
	}

	return &AuthClient{
		client: client,
	}, nil
}

// GetSecrets fetches secrets from cloudflare-auth-worker
func (c *AuthClient) GetSecrets(processName string) (map[string]string, error) {
	// Authenticate with Cloudflare Worker
	result, err := c.client.Authenticate()
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Extract secret data from response
	if !result.Success {
		return nil, fmt.Errorf("authentication unsuccessful: %s", result.Error)
	}

	// SecretData is already map[string]string from go_auth
	return result.SecretData, nil
}

// Health checks the health of the Cloudflare Worker
func (c *AuthClient) Health() error {
	resp, err := c.client.Health()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("worker unhealthy: status=%s", resp.Status)
	}

	return nil
}

// loadPrivateKey loads an RSA private key from a PEM file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not RSA private key")
		}
	}

	return key, nil
}
