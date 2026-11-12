package service

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestToPublicEmailConfig_HidesSecrets(t *testing.T) {
	cfg := &model.EmailConfig{
		Enabled:         true,
		SMTPHost:        "smtp.example.com",
		SMTPPassword:    "smtp-pass",
		EmailCodeSecret: "code-secret",
	}
	pub := ToPublicEmailConfig(cfg, true)
	if pub.SMTPPassword != "" || pub.EmailCodeSecret != "" {
		t.Fatalf("secrets leaked: %+v", pub)
	}
	if !pub.SMTPPasswordSet || !pub.EmailCodeSecretSet || !pub.Available {
		t.Fatalf("flags incorrect: %+v", pub)
	}
}

func TestToPublicStorageConfig_HidesSecrets(t *testing.T) {
	cfg := &model.StorageConfig{
		ProviderType: "s3",
		AccessKey:    "AKIA123",
		SecretKey:    "secret",
		Bucket:       "b",
	}
	pub := ToPublicStorageConfig(cfg)
	if pub.AccessKey != "" || pub.SecretKey != "" {
		t.Fatalf("secrets leaked: %+v", pub)
	}
	if !pub.AccessKeySet || !pub.SecretKeySet {
		t.Fatalf("flags incorrect: %+v", pub)
	}
}
