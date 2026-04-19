package unittests

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"sync"
	"testing"

	"helios-backend/internal/crypto"
)

var cryptoInitOnce sync.Once

func ensureCryptoReady(t *testing.T) {
	t.Helper()
	cryptoInitOnce.Do(func() {
		k := make([]byte, 32)
		_, _ = rand.Read(k)
		os.Setenv(crypto.EnvKey, base64.StdEncoding.EncodeToString(k))
		if err := crypto.Init(); err != nil {
			t.Fatalf("crypto init: %v", err)
		}
	})
}

func TestCrypto_RoundTrip(t *testing.T) {
	ensureCryptoReady(t)
	plain := []byte("sensitive complaint note 🔒")
	blob, err := crypto.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(blob, plain) {
		t.Fatal("blob must not contain plaintext")
	}
	out, err := crypto.Decrypt(blob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("round trip: got %q", out)
	}
}

func TestCrypto_NonceIsFresh(t *testing.T) {
	ensureCryptoReady(t)
	a, _ := crypto.Encrypt([]byte("abc"))
	b, _ := crypto.Encrypt([]byte("abc"))
	if bytes.Equal(a, b) {
		t.Fatal("ciphertexts must differ (fresh nonce)")
	}
}

func TestCrypto_TamperDetected(t *testing.T) {
	ensureCryptoReady(t)
	blob, _ := crypto.Encrypt([]byte("payload"))
	blob[len(blob)-1] ^= 0x01
	if _, err := crypto.Decrypt(blob); err == nil {
		t.Fatal("expected GCM auth failure on tampered blob")
	}
}

func TestCrypto_SchemeName(t *testing.T) {
	if crypto.SchemeName != "aes-256-gcm" {
		t.Fatalf("scheme drifted: %q", crypto.SchemeName)
	}
}
