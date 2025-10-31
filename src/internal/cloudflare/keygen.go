package cloudflare

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateKeyPair generates RSA key pair and saves to files
func GenerateKeyPair(privateKeyPath, publicKeyPath string, bits int) error {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create directories if they don't exist
	privateKeyDir := filepath.Dir(privateKeyPath)
	if err := os.MkdirAll(privateKeyDir, 0755); err != nil {
		return fmt.Errorf("failed to create private key directory: %w", err)
	}

	publicKeyDir := filepath.Dir(publicKeyPath)
	if err := os.MkdirAll(publicKeyDir, 0755); err != nil {
		return fmt.Errorf("failed to create public key directory: %w", err)
	}

	// Save private key
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Save public key
	publicKeyFile, err := os.Create(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer publicKeyFile.Close()

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// EnsureKeyPairExists checks if key pair exists, generates if not
func EnsureKeyPairExists(privateKeyPath, publicKeyPath string, bits int) error {
	// Check if private key exists
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		fmt.Printf("Private key not found at %s, generating new key pair...\n", privateKeyPath)
		return GenerateKeyPair(privateKeyPath, publicKeyPath, bits)
	}

	// Check if public key exists
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		fmt.Printf("Public key not found at %s, generating new key pair...\n", publicKeyPath)
		return GenerateKeyPair(privateKeyPath, publicKeyPath, bits)
	}

	return nil
}
