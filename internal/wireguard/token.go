package wireguard

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// tokenVersion is the wire format version for JoinToken and EnrollmentReply.
const tokenVersion = 1

// JoinToken is the public enrollment blob a server hands to a prospective
// client. It contains no secrets, so it may be transferred over any channel.
type JoinToken struct {
	Version         int    `json:"v"`
	ServerPublicKey string `json:"spk"`
	ServerAddress   string `json:"saddr"`
	Endpoint        string `json:"ep"`
	Subnet          string `json:"net"`
	ClientAddress   string `json:"caddr"`
	ListenPort      int    `json:"port"`
	Keepalive       int    `json:"ka"`
}

// EnrollmentReply is what a client returns to the server after consuming a
// token. It carries the client's public key and, if used, a preshared key —
// so this blob is SENSITIVE and should be moved over a trusted channel.
type EnrollmentReply struct {
	Version         int    `json:"v"`
	Name            string `json:"name"`
	ClientPublicKey string `json:"cpk"`
	ClientAddress   string `json:"caddr"`
	PresharedKey    string `json:"psk,omitempty"`
}

// Encode serializes a token as a URL-safe base64 string.
func (t JoinToken) Encode() (string, error) {
	return encodeJSON(t)
}

// DecodeToken parses and validates a base64 join token.
func DecodeToken(s string) (JoinToken, error) {
	var t JoinToken
	if err := decodeJSON(s, &t); err != nil {
		return JoinToken{}, fmt.Errorf("decode join token: %w", err)
	}
	if err := t.validate(); err != nil {
		return JoinToken{}, err
	}
	return t, nil
}

func (t JoinToken) validate() error {
	if t.Version != tokenVersion {
		return fmt.Errorf("unsupported token version %d (expected %d)", t.Version, tokenVersion)
	}
	for field, val := range map[string]string{
		"server public key": t.ServerPublicKey,
		"server address":    t.ServerAddress,
		"subnet":            t.Subnet,
		"client address":    t.ClientAddress,
	} {
		if val == "" {
			return fmt.Errorf("join token is missing the %s", field)
		}
	}
	return nil
}

// Encode serializes a reply as a URL-safe base64 string.
func (r EnrollmentReply) Encode() (string, error) {
	return encodeJSON(r)
}

// DecodeReply parses and validates a base64 enrollment reply.
func DecodeReply(s string) (EnrollmentReply, error) {
	var r EnrollmentReply
	if err := decodeJSON(s, &r); err != nil {
		return EnrollmentReply{}, fmt.Errorf("decode enrollment reply: %w", err)
	}
	if r.Version != tokenVersion {
		return EnrollmentReply{}, fmt.Errorf("unsupported reply version %d (expected %d)", r.Version, tokenVersion)
	}
	if r.ClientPublicKey == "" || r.ClientAddress == "" {
		return EnrollmentReply{}, fmt.Errorf("enrollment reply is missing the client public key or address")
	}
	return r, nil
}

func encodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeJSON(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
