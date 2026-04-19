package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	k := make([]byte, keySize)
	_, _ = rand.Read(k)
	os.Setenv(EnvKey, base64.StdEncoding.EncodeToString(k))
	if err := Init(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plain := []byte("this is a secret complaint note 🔒")
	blob, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(blob, plain) {
		t.Fatal("blob should not contain plaintext")
	}
	out, err := Decrypt(blob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("round trip mismatch: got %q", out)
	}
}

func TestEncrypt_NonceDiffersEachCall(t *testing.T) {
	a, _ := Encrypt([]byte("abc"))
	b, _ := Encrypt([]byte("abc"))
	if bytes.Equal(a, b) {
		t.Fatal("ciphertext should differ due to fresh nonce")
	}
}

func TestDecrypt_TamperedBlobFails(t *testing.T) {
	blob, _ := Encrypt([]byte("payload"))
	// Flip one bit in the tag/ciphertext region
	blob[len(blob)-1] ^= 0x01
	if _, err := Decrypt(blob); err == nil {
		t.Fatal("expected GCM auth failure on tampered blob")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	if _, err := Decrypt([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error on too-short blob")
	}
}
