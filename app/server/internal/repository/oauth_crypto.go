package repository

import (
	"strings"

	"GOSpeak/internal/model"
	"GOSpeak/internal/storage"
)

const oauthSecretPrefix = "enc:v1:"

func encryptOAuthSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if strings.HasPrefix(plaintext, oauthSecretPrefix) {
		if _, err := storage.DecryptSecret(strings.TrimPrefix(plaintext, oauthSecretPrefix)); err == nil {
			return plaintext, nil
		}
	}
	encoded, err := storage.EncryptSecret(plaintext)
	if err != nil {
		return "", err
	}
	return oauthSecretPrefix + encoded, nil
}

func decryptOAuthSecret(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	return storage.DecryptSecret(strings.TrimPrefix(encoded, oauthSecretPrefix))
}

func encryptOAuthProviderSecrets(p *model.OAuthProvider) error {
	if p == nil {
		return nil
	}
	secret, err := encryptOAuthSecret(p.ClientSecret)
	if err != nil {
		return err
	}
	p.ClientSecret = secret
	return nil
}

func decryptOAuthProviderSecrets(p *model.OAuthProvider) error {
	if p == nil {
		return nil
	}
	secret, err := decryptOAuthSecret(p.ClientSecret)
	if err != nil {
		return err
	}
	p.ClientSecret = secret
	return nil
}

func encryptOAuthAccountSecrets(a *model.OAuthAccount) error {
	if a == nil {
		return nil
	}
	accessToken, err := encryptOAuthSecret(a.AccessToken)
	if err != nil {
		return err
	}
	refreshToken, err := encryptOAuthSecret(a.RefreshToken)
	if err != nil {
		return err
	}
	a.AccessToken = accessToken
	a.RefreshToken = refreshToken
	return nil
}

func decryptOAuthAccountSecrets(a *model.OAuthAccount) error {
	if a == nil {
		return nil
	}
	accessToken, err := decryptOAuthSecret(a.AccessToken)
	if err != nil {
		return err
	}
	refreshToken, err := decryptOAuthSecret(a.RefreshToken)
	if err != nil {
		return err
	}
	a.AccessToken = accessToken
	a.RefreshToken = refreshToken
	return nil
}
