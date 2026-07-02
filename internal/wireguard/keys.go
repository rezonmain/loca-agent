// Package wireguard automates WireGuard key generation, tunnel address
// assignment, and configuration rendering. It has no side effects on the host:
// it produces keys, addresses, and .conf text that callers (the installers)
// write to disk and hand to the WireGuard tooling.
package wireguard

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Keypair is a base64-encoded Curve25519 key pair in WireGuard's format.
type Keypair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateKeypair creates a new Curve25519 key pair. The private key is clamped
// per RFC 7748 so it matches the output of `wg genkey`.
func GenerateKeypair() (Keypair, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Keypair{}, fmt.Errorf("generate x25519 key: %w", err)
	}
	return Keypair{
		PrivateKey: base64.StdEncoding.EncodeToString(clamp(priv.Bytes())),
		PublicKey:  base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
	}, nil
}

// GeneratePresharedKey creates a random 32-byte preshared key (base64).
func GeneratePresharedKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate preshared key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

// PublicFromPrivate derives the public key for a base64-encoded private key.
func PublicFromPrivate(privB64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes, got %d", len(raw))
	}
	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", fmt.Errorf("load private key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()), nil
}

// clamp applies the Curve25519 scalar clamping used by WireGuard.
func clamp(k []byte) []byte {
	c := make([]byte, len(k))
	copy(c, k)
	c[0] &= 248
	c[31] &= 127
	c[31] |= 64
	return c
}
