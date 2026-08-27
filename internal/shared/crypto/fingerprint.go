package crypto

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"strings"
)

// Fingerprint returns the SHA-256 fingerprint of a certificate's DER
// encoding, formatted as lowercase hex grouped in pairs separated by ':'
// (SSH-key-fingerprint style), e.g. "a1:2b:...". This is the value shown to
// a human for manual TOFU confirmation and the value pinned in storage.
func Fingerprint(cert *x509.Certificate) string {
	return FingerprintFromDER(cert.Raw)
}

// FingerprintFromDER fingerprints a raw DER-encoded certificate, for
// callers that only have access to the raw bytes presented during a TLS
// handshake (e.g. tls.Config.VerifyPeerCertificate's rawCerts).
func FingerprintFromDER(der []byte) string {
	sum := sha256.Sum256(der)
	hexStr := hex.EncodeToString(sum[:])

	groups := make([]string, 0, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		groups = append(groups, hexStr[i:i+2])
	}
	return strings.Join(groups, ":")
}
