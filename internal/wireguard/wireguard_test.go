package wireguard

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateKeypairConsistency(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	priv, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil || len(priv) != 32 {
		t.Fatalf("private key not 32 base64 bytes: len=%d err=%v", len(priv), err)
	}
	pub, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil || len(pub) != 32 {
		t.Fatalf("public key not 32 base64 bytes: len=%d err=%v", len(pub), err)
	}

	// Clamping invariants of the private scalar.
	if priv[0]&7 != 0 || priv[31]&128 != 0 || priv[31]&64 == 0 {
		t.Errorf("private key is not clamped per RFC 7748")
	}

	// The public key must derive deterministically from the private key.
	derived, err := PublicFromPrivate(kp.PrivateKey)
	if err != nil {
		t.Fatalf("PublicFromPrivate: %v", err)
	}
	if derived != kp.PublicKey {
		t.Errorf("derived public %q != generated public %q", derived, kp.PublicKey)
	}
}

func TestGenerateKeypairUnique(t *testing.T) {
	a, _ := GenerateKeypair()
	b, _ := GenerateKeypair()
	if a.PrivateKey == b.PrivateKey {
		t.Errorf("two generated private keys are identical")
	}
}

func TestNextClientAddress(t *testing.T) {
	tests := []struct {
		name   string
		subnet string
		taken  []string
		want   string
	}{
		{"first host", "10.50.0.0/24", nil, "10.50.0.1"},
		{"skip taken", "10.50.0.0/24", []string{"10.50.0.1", "10.50.0.2"}, "10.50.0.3"},
		{"gap reused", "10.50.0.0/24", []string{"10.50.0.1", "10.50.0.3"}, "10.50.0.2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NextClientAddress(tc.subnet, tc.taken)
			if err != nil {
				t.Fatalf("NextClientAddress: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNextClientAddressExhausted(t *testing.T) {
	// A /32 has no host address once the network address is skipped.
	if _, err := NextClientAddress("10.0.0.0/32", []string{}); err == nil {
		t.Errorf("expected error for /32 with no usable hosts")
	}
}

// server sets up a ServerIdentity for tests.
func testServer(t *testing.T) ServerIdentity {
	t.Helper()
	return ServerIdentity{
		Name:       "windows-server",
		Keys:       mustKeys(t),
		Address:    "10.50.0.1",
		Subnet:     "10.50.0.0/24",
		ListenPort: 51820,
		Endpoint:   "vpn.example.com:51820",
		Keepalive:  25,
	}
}

// TestEnrollmentRoundTrip exercises the full token → enroll → reply → add-peer
// flow and asserts that no private key crosses machines.
func TestEnrollmentRoundTrip(t *testing.T) {
	srv := testServer(t)

	// Server allocates an address and mints a token.
	clientAddr, err := srv.NextClientAddress(nil)
	if err != nil {
		t.Fatalf("NextClientAddress: %v", err)
	}
	if clientAddr != "10.50.0.2" {
		t.Fatalf("first client address = %q, want 10.50.0.2", clientAddr)
	}
	tokenStr, err := srv.MintToken(clientAddr).Encode()
	if err != nil {
		t.Fatalf("Encode token: %v", err)
	}

	// Client decodes the token and enrolls locally with a PSK.
	token, err := DecodeToken(tokenStr)
	if err != nil {
		t.Fatalf("DecodeToken: %v", err)
	}
	enr, err := Enroll(token, "mac-client", true)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	clientConf, err := Render(enr.Config)
	if err != nil {
		t.Fatalf("Render client: %v", err)
	}
	assertContains(t, clientConf, "PrivateKey = "+enr.Keys.PrivateKey)
	assertContains(t, clientConf, "Address = 10.50.0.2/24")
	assertContains(t, clientConf, "PublicKey = "+srv.Keys.PublicKey)
	assertContains(t, clientConf, "AllowedIPs = 10.50.0.1/32")
	assertContains(t, clientConf, "Endpoint = vpn.example.com:51820")
	assertContains(t, clientConf, "PersistentKeepalive = 25")
	assertContains(t, clientConf, "PresharedKey = "+enr.Reply.PresharedKey)
	assertNotContains(t, clientConf, "ListenPort")        // clients do not listen
	assertNotContains(t, clientConf, srv.Keys.PrivateKey) // server private key never travels

	// Server registers the client from its reply.
	replyStr, err := enr.Reply.Encode()
	if err != nil {
		t.Fatalf("Encode reply: %v", err)
	}
	reply, err := DecodeReply(replyStr)
	if err != nil {
		t.Fatalf("DecodeReply: %v", err)
	}
	serverCfg, err := srv.Config([]ServerPeer{PeerFromReply(reply)})
	if err != nil {
		t.Fatalf("server Config: %v", err)
	}
	serverConf, err := Render(serverCfg)
	if err != nil {
		t.Fatalf("Render server: %v", err)
	}
	assertContains(t, serverConf, "PrivateKey = "+srv.Keys.PrivateKey)
	assertContains(t, serverConf, "Address = 10.50.0.1/24")
	assertContains(t, serverConf, "ListenPort = 51820")
	assertContains(t, serverConf, "PublicKey = "+enr.Keys.PublicKey)
	assertContains(t, serverConf, "AllowedIPs = 10.50.0.2/32")
	assertContains(t, serverConf, "PresharedKey = "+enr.Reply.PresharedKey)
	assertNotContains(t, serverConf, enr.Keys.PrivateKey) // client private key never travels
}

func TestTokenIsPublicReplyCarriesPSK(t *testing.T) {
	srv := testServer(t)
	tokenStr, _ := srv.MintToken("10.50.0.2").Encode()

	// A decoded token must never contain the server's private key.
	if strings.Contains(tokenStr, srv.Keys.PrivateKey) {
		t.Errorf("join token leaked the server private key")
	}

	enr, err := Enroll(srv.MintToken("10.50.0.2"), "c", true)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if enr.Reply.PresharedKey == "" {
		t.Errorf("expected reply to carry a preshared key when withPSK=true")
	}

	noPSK, _ := Enroll(srv.MintToken("10.50.0.2"), "c", false)
	if noPSK.Reply.PresharedKey != "" {
		t.Errorf("expected no preshared key when withPSK=false")
	}
}

func TestDecodeTokenRejectsGarbage(t *testing.T) {
	if _, err := DecodeToken("!!!not base64!!!"); err == nil {
		t.Errorf("expected error decoding malformed token")
	}
	if _, err := DecodeToken(""); err == nil {
		t.Errorf("expected error decoding empty token")
	}
}

func mustKeys(t *testing.T) Keypair {
	t.Helper()
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	return kp
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q\n---\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q\n---\n%s", needle, haystack)
	}
}
