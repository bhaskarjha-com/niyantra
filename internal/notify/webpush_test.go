package notify

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateVAPIDKeys(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys: %v", err)
	}
	if pub == "" || priv == "" {
		t.Fatal("expected non-empty keys")
	}

	// Public key should decode to 65 bytes (uncompressed P-256 point)
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(pubBytes) != 65 {
		t.Errorf("public key length = %d, want 65", len(pubBytes))
	}
	if pubBytes[0] != 0x04 {
		t.Errorf("public key prefix = %x, want 0x04", pubBytes[0])
	}

	// Private key should decode to 32 bytes
	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("private key length = %d, want 32", len(privBytes))
	}
}

func TestGenerateVAPIDKeysUniqueness(t *testing.T) {
	pub1, _, _ := GenerateVAPIDKeys()
	pub2, _, _ := GenerateVAPIDKeys()
	if pub1 == pub2 {
		t.Error("two generated key pairs should be unique")
	}
}

func TestWebPushConfigIsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  WebPushConfig
		want bool
	}{
		{"full", WebPushConfig{Enabled: true, PublicKey: "pub", PrivateKey: "priv"}, true},
		{"disabled", WebPushConfig{Enabled: false, PublicKey: "pub", PrivateKey: "priv"}, false},
		{"no pub", WebPushConfig{Enabled: true, PublicKey: "", PrivateKey: "priv"}, false},
		{"no priv", WebPushConfig{Enabled: true, PublicKey: "pub", PrivateKey: ""}, false},
		{"empty", WebPushConfig{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHKDFDerive(t *testing.T) {
	ikm := []byte("input key material")
	salt := []byte("salt value here!")
	info := []byte("application info")

	key16 := hkdfDerive(ikm, salt, info, 16)
	if len(key16) != 16 {
		t.Errorf("hkdfDerive length = %d, want 16", len(key16))
	}

	key32 := hkdfDerive(ikm, salt, info, 32)
	if len(key32) != 32 {
		t.Errorf("hkdfDerive length = %d, want 32", len(key32))
	}

	// Same inputs should produce same output
	key16b := hkdfDerive(ikm, salt, info, 16)
	if string(key16) != string(key16b) {
		t.Error("same inputs should produce same output")
	}

	// Different info should produce different output
	key16c := hkdfDerive(ikm, salt, []byte("different info"), 16)
	if string(key16) == string(key16c) {
		t.Error("different info should produce different output")
	}
}

func TestHKDFDeriveEmptySalt(t *testing.T) {
	ikm := []byte("test")
	key := hkdfDerive(ikm, nil, []byte("info"), 16)
	if len(key) != 16 {
		t.Errorf("hkdfDerive with nil salt: length = %d, want 16", len(key))
	}
}

func TestDecodeB64(t *testing.T) {
	original := []byte("hello world test data")

	// Test raw URL encoding
	encoded := base64.RawURLEncoding.EncodeToString(original)
	decoded, err := decodeB64(encoded)
	if err != nil {
		t.Fatalf("decodeB64 raw URL: %v", err)
	}
	if string(decoded) != string(original) {
		t.Errorf("decoded = %q, want %q", decoded, original)
	}

	// Test standard encoding
	encodedStd := base64.StdEncoding.EncodeToString(original)
	decoded2, err := decodeB64(encodedStd)
	if err != nil {
		t.Fatalf("decodeB64 standard: %v", err)
	}
	if string(decoded2) != string(original) {
		t.Errorf("decoded = %q, want %q", decoded2, original)
	}
}

func TestBuildVAPIDAuth(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	pubB64 := base64.RawURLEncoding.EncodeToString(pubBytes)

	auth, err := buildVAPIDAuth("https://fcm.googleapis.com/push/123", privKey, pubB64)
	if err != nil {
		t.Fatalf("buildVAPIDAuth: %v", err)
	}

	// Should start with "vapid t=..."
	if !wpStartsWith(auth, "vapid t=") {
		t.Errorf("auth header should start with 'vapid t=', got %q", auth[:20])
	}

	// Should contain ", k="
	if !wpContains(auth, ", k="+pubB64) {
		t.Error("auth header should contain public key")
	}
}

func TestSendWebPushNotConfigured(t *testing.T) {
	cfg := &WebPushConfig{Enabled: false}
	sub := &WebPushSubscription{Endpoint: "https://example.com"}
	if err := SendWebPush(cfg, sub, []byte("test")); err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendWebPushToTestServer(t *testing.T) {
	// Create a mock push service
	var receivedBody []byte
	var receivedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(201) // Push services return 201
	}))
	defer srv.Close()

	// Generate VAPID keys
	pubB64, privB64, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}

	// Generate a "client" key pair (simulating browser)
	clientPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientPubBytes := elliptic.Marshal(elliptic.P256(), clientPriv.PublicKey.X, clientPriv.PublicKey.Y)
	clientPubB64 := base64.RawURLEncoding.EncodeToString(clientPubBytes)

	authSecret := make([]byte, 16)
	rand.Read(authSecret)
	authB64 := base64.RawURLEncoding.EncodeToString(authSecret)

	cfg := &WebPushConfig{
		Enabled:    true,
		PublicKey:  pubB64,
		PrivateKey: privB64,
	}
	sub := &WebPushSubscription{
		Endpoint: srv.URL,
		Keys: WebPushKeys{
			Auth:   authB64,
			P256dh: clientPubB64,
		},
	}

	err = SendWebPush(cfg, sub, []byte(`{"title":"Test","body":"Hello"}`))
	if err != nil {
		t.Fatalf("SendWebPush: %v", err)
	}

	// Verify headers
	if receivedHeaders.Get("Content-Encoding") != "aes128gcm" {
		t.Errorf("Content-Encoding = %q, want aes128gcm", receivedHeaders.Get("Content-Encoding"))
	}
	if receivedHeaders.Get("TTL") != "2419200" {
		t.Errorf("TTL = %q, want 2419200", receivedHeaders.Get("TTL"))
	}
	if receivedHeaders.Get("Urgency") != "high" {
		t.Errorf("Urgency = %q, want high", receivedHeaders.Get("Urgency"))
	}
	auth := receivedHeaders.Get("Authorization")
	if !wpStartsWith(auth, "vapid t=") {
		t.Errorf("Authorization should start with 'vapid t=', got %q", auth)
	}

	// Body should be non-empty (encrypted payload)
	if len(receivedBody) == 0 {
		t.Error("expected non-empty encrypted body")
	}
}

func TestSendWebPushHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(410) // Gone — subscription expired
	}))
	defer srv.Close()

	pubB64, privB64, _ := GenerateVAPIDKeys()
	clientPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	clientPubBytes := elliptic.Marshal(elliptic.P256(), clientPriv.PublicKey.X, clientPriv.PublicKey.Y)

	authSecret := make([]byte, 16)
	rand.Read(authSecret)

	cfg := &WebPushConfig{Enabled: true, PublicKey: pubB64, PrivateKey: privB64}
	sub := &WebPushSubscription{
		Endpoint: srv.URL,
		Keys: WebPushKeys{
			Auth:   base64.RawURLEncoding.EncodeToString(authSecret),
			P256dh: base64.RawURLEncoding.EncodeToString(clientPubBytes),
		},
	}

	err := SendWebPush(cfg, sub, []byte("test"))
	if err == nil {
		t.Error("expected error on HTTP 410")
	}
}

func TestSendTestWebPushNotConfigured(t *testing.T) {
	cfg := &WebPushConfig{Enabled: false}
	if err := SendTestWebPush(cfg, nil); err == nil {
		t.Error("expected error when not configured")
	}
}

func TestSendTestWebPushNoSubscriptions(t *testing.T) {
	cfg := &WebPushConfig{Enabled: true, PublicKey: "pub", PrivateKey: "priv"}
	if err := SendTestWebPush(cfg, nil); err == nil {
		t.Error("expected error with no subscriptions")
	}
}

func TestFormatQuotaAlertPush(t *testing.T) {
	payload := FormatQuotaAlertPush("claude-3.5-sonnet", 8.5, 10)
	var data map[string]string
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data["title"] == "" {
		t.Error("expected non-empty title")
	}
	if data["body"] == "" {
		t.Error("expected non-empty body")
	}
}

// helpers
func wpStartsWith(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }
func wpContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
