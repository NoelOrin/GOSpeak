package botbase

import (
	"GOSpeak/internal/storage"
	"strings"
)

const secretPrefix = "enc:v1:"

func encryptSecretValue(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, secretPrefix) {
		if _, err := storage.DecryptSecret(strings.TrimPrefix(plaintext, secretPrefix)); err == nil {
			return plaintext, nil
		}
	}
	encoded, err := storage.EncryptSecret(plaintext)
	if err != nil {
		return "", err
	}
	return secretPrefix + encoded, nil
}

func decryptSecretValue(encoded string) string {
	if encoded == "" || !strings.HasPrefix(encoded, secretPrefix) {
		return encoded
	}
	if plain, err := storage.DecryptSecret(strings.TrimPrefix(encoded, secretPrefix)); err == nil {
		return plain
	}
	return encoded
}

func encryptConfigSecrets(m map[string]any) (map[string]any, error) {
	providers, _ := m["llm_providers"].([]any)
	for _, item := range providers {
		p, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if key, ok := p["api_key"].(string); ok {
			enc, err := encryptSecretValue(key)
			if err != nil {
				return nil, err
			}
			p["api_key"] = enc
		}
	}
	return m, nil
}

func decryptConfigSecrets(m map[string]any) map[string]any {
	m = copyConfigMap(m)
	providers, _ := m["llm_providers"].([]any)
	for _, item := range providers {
		p, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if key, ok := p["api_key"].(string); ok {
			p["api_key"] = decryptSecretValue(key)
		}
	}
	return m
}

// copyConfigMap 返回顶层 map 与 llm_providers 的浅拷贝，避免加解密原地改写调用方配置。
func copyConfigMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	raw, ok := out["llm_providers"].([]any)
	if !ok {
		return out
	}
	providers := make([]any, len(raw))
	for i, item := range raw {
		if p, ok := item.(map[string]any); ok {
			cp := make(map[string]any, len(p))
			for k, v := range p {
				cp[k] = v
			}
			providers[i] = cp
		} else {
			providers[i] = item
		}
	}
	out["llm_providers"] = providers
	return out
}
