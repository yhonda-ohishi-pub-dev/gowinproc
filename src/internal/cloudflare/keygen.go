package cloudflare

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Generate Cloudflare config JSON
	if err := generateCloudflareJSON(publicKeyPath, "gowinproc"); err != nil {
		fmt.Printf("Warning: failed to generate Cloudflare JSON: %v\n", err)
	}

	return nil
}

// generateCloudflareJSON generates JSON file for Cloudflare Workers AUTHORIZED_CLIENTS
func generateCloudflareJSON(publicKeyPath, clientID string) error {
	// Read public key
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Keep newlines as-is - JSON marshaler will handle escaping
	publicKeyStr := strings.TrimSpace(string(publicKeyData))

	// Create JSON object in "client-id: key" format
	config := map[string]string{
		clientID: publicKeyStr,
	}

	// Write to .cloudflare.json file
	jsonPath := publicKeyPath + ".cloudflare.json"
	jsonFile, err := os.Create(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer jsonFile.Close()

	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	fmt.Printf("\n")
	fmt.Printf("=================================================================\n")
	fmt.Printf("Cloudflare Workers設定用JSON (AUTHORIZED_CLIENTS配列に追加):\n")
	fmt.Printf("=================================================================\n")
	fmt.Printf("ファイル: %s\n\n", jsonPath)

	// Also print to stdout for easy copy-paste
	jsonBytes, _ := json.MarshalIndent(config, "", "  ")
	fmt.Printf("%s\n", string(jsonBytes))
	fmt.Printf("=================================================================\n\n")

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
