// Package crypto provides the self-signed identity, fingerprinting and
// mTLS helpers shared between the homectl daemon and client. There is no
// real PKI: every party generates its own Ed25519 key pair and a
// self-signed certificate on first run, and trust is established purely by
// pinning the peer's certificate fingerprint (TOFU).
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	keyFileName  = "identity.key"
	certFileName = "identity.crt"

	certValidity = 10 * 365 * 24 * time.Hour
)

// Identity is a self-signed Ed25519 identity: the certificate and key an
// endpoint presents during the mTLS handshake.
type Identity struct {
	PrivateKey  ed25519.PrivateKey
	Certificate *x509.Certificate
	TLS         tls.Certificate
}

// Fingerprint returns this identity's own certificate fingerprint.
func (id *Identity) Fingerprint() string {
	return Fingerprint(id.Certificate)
}

// LoadOrCreateIdentity loads the identity stored in dir, generating and
// persisting a new one (with restrictive file permissions) if none exists
// yet. commonName is embedded in the self-signed certificate purely for
// human-readable debugging (e.g. "homectl-daemon@myhost"); it plays no role
// in trust decisions, which are fingerprint-based only.
func LoadOrCreateIdentity(dir, commonName string) (*Identity, error) {
	keyPath := filepath.Join(dir, keyFileName)
	certPath := filepath.Join(dir, certFileName)

	if _, err := os.Stat(keyPath); err == nil {
		return loadIdentity(keyPath, certPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat identity key: %w", err)
	}

	return createIdentity(dir, keyPath, certPath, commonName)
}

func loadIdentity(keyPath, certPath string) (*Identity, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read identity key: %w", err)
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read identity certificate: %w", err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse identity key pair: %w", err)
	}
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse identity certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("decode identity key PEM")
	}
	priv, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse identity private key: %w", err)
	}
	edPriv, ok := priv.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("identity private key is not Ed25519")
	}

	return &Identity{PrivateKey: edPriv, Certificate: cert, TLS: tlsCert}, nil
}

func createIdentity(dir, keyPath, certPath, commonName string) (*Identity, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create identity dir: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Ed25519 key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(certValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:         false,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("create self-signed certificate: %w", err)
	}

	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write identity key: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return nil, fmt.Errorf("write identity certificate: %w", err)
	}

	return loadIdentity(keyPath, certPath)
}
