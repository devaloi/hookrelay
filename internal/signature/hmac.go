// Package signature provides HMAC-SHA256 signing and verification for webhooks.
package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// SignatureHeader is the HTTP header name for the webhook signature.
	SignatureHeader = "X-Hook-Signature"

	// signaturePrefix is the prefix for SHA256 signatures.
	signaturePrefix = "sha256="
)

// Sign generates an HMAC-SHA256 signature for the given payload using the secret.
func Sign(payload []byte, secret string) string {
	if secret == "" {
		return ""
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	return signaturePrefix + signature
}

// Verify checks if the provided signature matches the payload signed with the secret.
func Verify(payload []byte, signature, secret string) error {
	if secret == "" {
		return nil
	}

	if signature == "" {
		return fmt.Errorf("missing signature header")
	}

	if !strings.HasPrefix(signature, signaturePrefix) {
		return fmt.Errorf("invalid signature format: expected %s prefix", signaturePrefix)
	}

	providedSig := strings.TrimPrefix(signature, signaturePrefix)
	providedBytes, err := hex.DecodeString(providedSig)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedBytes := mac.Sum(nil)

	if !hmac.Equal(providedBytes, expectedBytes) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
