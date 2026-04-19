package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const (
	keySize    = 32 // AES-256
	SchemeName = "aes-256-gcm"
	EnvKey     = "HELIOS_CRYPTO_KEY"
	EnvKeyPath = "HELIOS_CRYPTO_KEY_PATH"
)

var (
	key     []byte
	initMu  sync.Mutex
	loaded  bool
)

// Init loads the symmetric key. Precedence: HELIOS_CRYPTO_KEY (base64 32 bytes)
// then HELIOS_CRYPTO_KEY_PATH (persisted 32-byte binary file; generated on
// first boot if missing).
func Init() error {
	initMu.Lock()
	defer initMu.Unlock()
	if loaded {
		return nil
	}

	if b64 := os.Getenv(EnvKey); b64 != "" {
		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return err
		}
		if len(b) != keySize {
			return errors.New("HELIOS_CRYPTO_KEY must decode to 32 bytes")
		}
		key = b
		loaded = true
		log.Println("crypto: loaded key from env HELIOS_CRYPTO_KEY")
		return nil
	}

	path := os.Getenv(EnvKeyPath)
	if path == "" {
		path = "/data/helios-crypto.key"
	}

	if b, err := os.ReadFile(path); err == nil {
		if len(b) != keySize {
			return errors.New("key file has wrong size: " + path)
		}
		key = b
		loaded = true
		log.Printf("crypto: loaded key from %s", path)
		return nil
	}

	// Generate new key
	k := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, k, 0o600); err != nil {
		return err
	}
	key = k
	loaded = true
	log.Printf("crypto: generated new key and stored at %s", path)
	return nil
}

func ensureLoaded() error {
	if !loaded || key == nil {
		return errors.New("crypto not initialized")
	}
	return nil
}

// Encrypt returns nonce || ciphertext || tag. Layout is a single opaque blob
// suitable for direct VARBINARY storage.
func Encrypt(plaintext []byte) ([]byte, error) {
	if err := ensureLoaded(); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func Decrypt(blob []byte) ([]byte, error) {
	if err := ensureLoaded(); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}
