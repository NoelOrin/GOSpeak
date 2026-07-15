package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
)

// devFallbackKey 仅用于开发环境，绝不可用于生产
const devFallbackKey = "0000000000000000000000000000000000000000000000000000000000000000"

// getEncryptKey 获取加密密钥（32字节 hex 编码 → 32字节）
func getEncryptKey() []byte {
	cfg := config.Current()
	hexKey := ""
	isDev := true
	if cfg != nil {
		hexKey = cfg.StorageEncryptKey
		isDev = !cfg.IsProduction()
	}

	if hexKey == "" {
		if isDev {
			logger.Warn("[Storage] ⚠️ STORAGE_ENCRYPT_KEY not set, using INSECURE dev fallback key — NEVER use in production")
			hexKey = devFallbackKey
		} else {
			logger.Fatal("[Storage] STORAGE_ENCRYPT_KEY is required in production. Set a 64-char hex string (32 bytes) and restart.")
		}
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		if isDev && hexKey == devFallbackKey {
			// dev fallback key is valid by definition, any error is a code bug
			logger.Fatal("[Storage] dev fallback key is invalid, this is a code bug")
		}
		logger.Fatal("[Storage] STORAGE_ENCRYPT_KEY must be a 64-char hex string (32 bytes). Got invalid key.")
	}
	return key
}

// EncryptSecret 使用 AES-256-GCM 加密，返回 base64 编码的密文
func EncryptSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := getEncryptKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM failed: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce failed: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret 解密 base64 编码的 AES-GCM 密文
func DecryptSecret(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	key := getEncryptKey()
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM failed: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}

	return string(plaintext), nil
}
