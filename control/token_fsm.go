package control

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

var errInvalidTokenFSM = errors.New("invalid token fsm")

var tokenFSMMagic = []byte{'F', 'S', 'M', '1'}

func SaveTokenToFSM(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("empty token")
	}

	payload, err := encryptTokenFSM(token)
	if err != nil {
		return err
	}
	return os.WriteFile(StorageTokenFilePath(), payload, 0600)
}

func LoadTokenFromFSM() (string, error) {
	payload, err := os.ReadFile(StorageTokenFilePath())
	if err != nil {
		return "", err
	}
	return decryptTokenFSM(payload)
}

func RemoveTokenFSM() error {
	err := os.Remove(StorageTokenFilePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func encryptTokenFSM(token string) ([]byte, error) {
	aead, err := newTokenFSMCipher()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(token), tokenFSMMagic)
	payload := make([]byte, 0, len(tokenFSMMagic)+len(nonce)+len(ciphertext))
	payload = append(payload, tokenFSMMagic...)
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)
	return payload, nil
}

func decryptTokenFSM(payload []byte) (string, error) {
	aead, err := newTokenFSMCipher()
	if err != nil {
		return "", err
	}

	headerSize := len(tokenFSMMagic) + aead.NonceSize()
	if len(payload) < headerSize {
		return "", errInvalidTokenFSM
	}
	if !bytes.Equal(payload[:len(tokenFSMMagic)], tokenFSMMagic) {
		return "", errInvalidTokenFSM
	}

	nonce := payload[len(tokenFSMMagic):headerSize]
	ciphertext := payload[headerSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, tokenFSMMagic)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidTokenFSM, err)
	}

	token := strings.TrimSpace(string(plaintext))
	if token == "" {
		return "", errInvalidTokenFSM
	}
	return token, nil
}

func newTokenFSMCipher() (cipher.AEAD, error) {
	key := tokenFSMKey()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return aead, nil
}

func tokenFSMKey() [32]byte {
	hostname, _ := os.Hostname()
	homeDir, _ := os.UserHomeDir()
	keyMaterial := strings.Join([]string{
		"NexusEgo",
		"token",
		"fsm",
		"v1",
		runtime.GOOS,
		runtime.GOARCH,
		hostname,
		homeDir,
	}, "|")
	return sha256.Sum256([]byte(keyMaterial))
}
