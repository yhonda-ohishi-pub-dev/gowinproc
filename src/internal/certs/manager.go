package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// Manager handles certificate generation and management
type Manager struct {
	certsDir string
	keysDir  string
}

// NewManager creates a new certificate manager
func NewManager(certsDir, keysDir string) (*Manager, error) {
	// Create directories if they don't exist
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create keys directory: %w", err)
	}

	return &Manager{
		certsDir: certsDir,
		keysDir:  keysDir,
	}, nil
}

// GenerateForProcess generates a self-signed certificate for a process
func (m *Manager) GenerateForProcess(processName string, hosts []string) (certPath, keyPath string, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"gowinproc"},
			CommonName:   processName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add DNS names
	for _, h := range hosts {
		template.DNSNames = append(template.DNSNames, h)
	}

	// Add localhost by default
	if len(template.DNSNames) == 0 {
		template.DNSNames = append(template.DNSNames, "localhost")
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate file
	certPath = filepath.Join(m.certsDir, fmt.Sprintf("%s.crt", processName))
	certFile, err := os.Create(certPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}); err != nil {
		return "", "", fmt.Errorf("failed to write cert file: %w", err)
	}

	// Write private key file
	keyPath = filepath.Join(m.keysDir, fmt.Sprintf("%s.key", processName))
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}); err != nil {
		return "", "", fmt.Errorf("failed to write key file: %w", err)
	}

	return certPath, keyPath, nil
}

// CertificateExists checks if a certificate already exists for a process
func (m *Manager) CertificateExists(processName string) bool {
	certPath := filepath.Join(m.certsDir, fmt.Sprintf("%s.crt", processName))
	keyPath := filepath.Join(m.keysDir, fmt.Sprintf("%s.key", processName))

	certInfo, certErr := os.Stat(certPath)
	keyInfo, keyErr := os.Stat(keyPath)

	return certErr == nil && keyErr == nil && !certInfo.IsDir() && !keyInfo.IsDir()
}

// GetPaths returns the certificate and key paths for a process
func (m *Manager) GetPaths(processName string) (certPath, keyPath string) {
	certPath = filepath.Join(m.certsDir, fmt.Sprintf("%s.crt", processName))
	keyPath = filepath.Join(m.keysDir, fmt.Sprintf("%s.key", processName))
	return
}

// RegenerateCertificate removes and regenerates a certificate
func (m *Manager) RegenerateCertificate(processName string, hosts []string) (certPath, keyPath string, err error) {
	certPath, keyPath = m.GetPaths(processName)

	// Remove existing files
	os.Remove(certPath)
	os.Remove(keyPath)

	// Generate new certificate
	return m.GenerateForProcess(processName, hosts)
}
