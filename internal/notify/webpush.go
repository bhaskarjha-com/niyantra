package notify

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebPushSubscription represents a PushSubscription from the Push API.
type WebPushSubscription struct {
	Endpoint string       `json:"endpoint"`
	Keys     WebPushKeys  `json:"keys"`
}

// WebPushKeys are the base64url-encoded keys from PushSubscription.getKey().
type WebPushKeys struct {
	Auth   string `json:"auth"`
	P256dh string `json:"p256dh"`
}

// WebPushConfig holds VAPID configuration for WebPush delivery.
type WebPushConfig struct {
	Enabled    bool
	PublicKey  string // base64url-encoded VAPID public key
	PrivateKey string // base64url-encoded VAPID private key
}

// IsConfigured returns true if VAPID keys are present and WebPush is enabled.
func (c *WebPushConfig) IsConfigured() bool {
	return c.Enabled && c.PublicKey != "" && c.PrivateKey != ""
}

// ── VAPID Key Generation ────────────────────────────────────────

// GenerateVAPIDKeys creates a new P-256 ECDSA key pair for VAPID.
// Returns (publicKey, privateKey) as base64url-encoded strings.
func GenerateVAPIDKeys() (string, string, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("webpush: generate key: %w", err)
	}

	// Public key: uncompressed EC point (65 bytes: 0x04 || X || Y)
	pubBytes := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)
	pubB64 := base64.RawURLEncoding.EncodeToString(pubBytes)

	// Private key: raw D value (32 bytes)
	privBytes := privateKey.D.Bytes()
	// Pad to 32 bytes if needed
	if len(privBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(privBytes):], privBytes)
		privBytes = padded
	}
	privB64 := base64.RawURLEncoding.EncodeToString(privBytes)

	return pubB64, privB64, nil
}

// ── WebPush Send ────────────────────────────────────────────────

// SendWebPush encrypts and sends a push notification to a subscription.
// Implements RFC 8291 (Message Encryption) + RFC 8292 (VAPID).
func SendWebPush(cfg *WebPushConfig, sub *WebPushSubscription, payload []byte) error {
	if !cfg.IsConfigured() {
		return fmt.Errorf("webpush: not configured")
	}

	// Decode subscription keys
	authSecret, err := decodeB64(sub.Keys.Auth)
	if err != nil {
		return fmt.Errorf("webpush: decode auth: %w", err)
	}
	clientPub, err := decodeB64(sub.Keys.P256dh)
	if err != nil {
		return fmt.Errorf("webpush: decode p256dh: %w", err)
	}

	// Decode VAPID private key
	privKeyBytes, err := decodeB64(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("webpush: decode private key: %w", err)
	}

	curve := elliptic.P256()

	// Reconstruct ECDSA private key
	d := new(big.Int).SetBytes(privKeyBytes)
	pubX, pubY := curve.ScalarBaseMult(d.Bytes())
	vapidPrivKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: pubX, Y: pubY},
		D:         d,
	}

	// ── RFC 8291: Encrypt payload ──

	// Generate ephemeral key pair
	ephPriv, ephX, ephY, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		return fmt.Errorf("webpush: gen ephemeral key: %w", err)
	}
	ephPub := elliptic.Marshal(curve, ephX, ephY)

	// ECDH shared secret: ephemeral private × client public
	cX, cY := elliptic.Unmarshal(curve, clientPub)
	if cX == nil {
		return fmt.Errorf("webpush: invalid client public key")
	}
	sx, _ := curve.ScalarMult(cX, cY, ephPriv)
	sharedSecret := make([]byte, 32)
	sx.FillBytes(sharedSecret)

	// IKM = HKDF(sharedSecret, authSecret, "WebPush: info\0" || clientPub || ephPub)
	info := append([]byte("WebPush: info\x00"), clientPub...)
	info = append(info, ephPub...)
	ikm := hkdfDerive(sharedSecret, authSecret, info, 32)

	// Generate 16 byte salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("webpush: gen salt: %w", err)
	}

	// CEK = HKDF(ikm, salt, "Content-Encoding: aes128gcm\0")
	cek := hkdfDerive(ikm, salt, []byte("Content-Encoding: aes128gcm\x00"), 16)

	// Nonce = HKDF(ikm, salt, "Content-Encoding: nonce\0")
	nonce := hkdfDerive(ikm, salt, []byte("Content-Encoding: nonce\x00"), 12)

	// AES-128-GCM encrypt
	block, err := aes.NewCipher(cek)
	if err != nil {
		return fmt.Errorf("webpush: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("webpush: gcm: %w", err)
	}

	// Pad payload: payload || 0x02 || zeros
	const recordSize uint32 = 4096
	padded := make([]byte, len(payload)+1)
	copy(padded, payload)
	padded[len(payload)] = 0x02 // delimiter

	ciphertext := gcm.Seal(nil, nonce, padded, nil)

	// Build aes128gcm content-coding header:
	// salt (16) || record_size (4) || key_id_len (1) || key_id (65)
	var body bytes.Buffer
	body.Write(salt)
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, recordSize)
	body.Write(rs)
	body.WriteByte(byte(len(ephPub)))
	body.Write(ephPub)
	body.Write(ciphertext)

	// ── RFC 8292: VAPID Authorization ──
	vapidAuth, err := buildVAPIDAuth(sub.Endpoint, vapidPrivKey, cfg.PublicKey)
	if err != nil {
		return fmt.Errorf("webpush: vapid: %w", err)
	}

	// ── HTTP POST ──
	req, err := http.NewRequest("POST", sub.Endpoint, &body)
	if err != nil {
		return fmt.Errorf("webpush: create request: %w", err)
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "2419200") // 28 days
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", vapidAuth)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webpush: send: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webpush: push service returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ── VAPID JWT ───────────────────────────────────────────────────

// buildVAPIDAuth constructs the VAPID Authorization header value.
func buildVAPIDAuth(endpoint string, privKey *ecdsa.PrivateKey, pubKeyB64 string) (string, error) {
	// Parse endpoint to get audience (origin)
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	audience := u.Scheme + "://" + u.Host

	// JWT header
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))

	// JWT claims
	claims := fmt.Sprintf(`{"aud":"%s","exp":%d,"sub":"mailto:niyantra@localhost"}`,
		audience, time.Now().Add(12*time.Hour).Unix())
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte(claims))

	// Sign
	signingInput := header + "." + claimsB64
	hash := sha256.Sum256([]byte(signingInput))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("webpush: sign: %w", err)
	}

	// Encode r, s as fixed 32-byte big-endian values concatenated
	curveBits := privKey.Curve.Params().BitSize
	keyBytes := curveBits / 8
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sig := make([]byte, 2*keyBytes)
	copy(sig[keyBytes-len(rBytes):keyBytes], rBytes)
	copy(sig[2*keyBytes-len(sBytes):], sBytes)

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	jwt := signingInput + "." + sigB64

	return fmt.Sprintf("vapid t=%s, k=%s", jwt, pubKeyB64), nil
}

// ── HKDF (RFC 5869) ─────────────────────────────────────────────

// hkdfDerive implements HKDF-Extract + HKDF-Expand using HMAC-SHA256.
// This avoids importing golang.org/x/crypto/hkdf.
func hkdfDerive(ikm, salt, info []byte, length int) []byte {
	// Extract: PRK = HMAC-SHA256(salt, ikm)
	if len(salt) == 0 {
		salt = make([]byte, sha256.Size)
	}
	extractor := hmac.New(sha256.New, salt)
	extractor.Write(ikm)
	prk := extractor.Sum(nil)

	// Expand: T(1) = HMAC-SHA256(PRK, info || 0x01)
	expander := hmac.New(sha256.New, prk)
	expander.Write(info)
	expander.Write([]byte{1})
	result := expander.Sum(nil)

	if length > len(result) {
		// Multi-block expansion (unlikely for our use — max 32 bytes)
		var prev []byte
		var out []byte
		for i := byte(1); len(out) < length; i++ {
			h := hmac.New(sha256.New, prk)
			h.Write(prev)
			h.Write(info)
			h.Write([]byte{i})
			prev = h.Sum(nil)
			out = append(out, prev...)
		}
		return out[:length]
	}

	return result[:length]
}

// ── Helpers ─────────────────────────────────────────────────────

// decodeB64 decodes a base64url or standard base64 string.
func decodeB64(s string) ([]byte, error) {
	// Try raw URL encoding first (no padding)
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// Try with padding
	padded := s
	if rem := len(s) % 4; rem != 0 {
		padded += strings.Repeat("=", 4-rem)
	}
	if b, err := base64.URLEncoding.DecodeString(padded); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(padded)
}

// SendTestWebPush sends a test push notification to verify the configuration.
func SendTestWebPush(cfg *WebPushConfig, subscriptions []WebPushSubscription) error {
	if !cfg.IsConfigured() {
		return fmt.Errorf("webpush: not configured")
	}
	if len(subscriptions) == 0 {
		return fmt.Errorf("webpush: no subscriptions registered — open the dashboard in a browser and enable push notifications first")
	}

	payload, _ := json.Marshal(map[string]string{
		"title": "Niyantra — Test Push",
		"body":  fmt.Sprintf("✅ WebPush notifications are working! Time: %s", time.Now().Format("15:04:05")),
	})

	var lastErr error
	for _, sub := range subscriptions {
		if err := SendWebPush(cfg, &sub, payload); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// FormatQuotaAlertPush creates a JSON payload for quota alert push notifications.
func FormatQuotaAlertPush(model string, remainingPct, threshold float64) []byte {
	payload, _ := json.Marshal(map[string]string{
		"title": fmt.Sprintf("⚠️ %s quota low", model),
		"body":  fmt.Sprintf("%.1f%% remaining (threshold: %.0f%%) — consider switching models", remainingPct, threshold),
	})
	return payload
}

// FormatDigestPush creates a JSON payload for digest push notifications (F8).
func FormatDigestPush(title, body string) []byte {
	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
	})
	return payload
}
