package oauth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

const secretToken = "SECRET_QQ_ACCESS_TOKEN"

func TestRedactURL_MasksSensitiveQuery(t *testing.T) {
	raw := "https://graph.qq.com/user/get_user_info?access_token=" + secretToken + "&fmt=json&oauth_token=OTHER"
	got := RedactURL(raw)
	if strings.Contains(got, secretToken) || strings.Contains(got, "OTHER") {
		t.Fatalf("sensitive query values survived: %q", got)
	}
	if !strings.Contains(got, "fmt=json") {
		t.Fatalf("non-sensitive query should be preserved: %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected placeholder, got %q", got)
	}
}

func TestRedactURL_DropsUserinfoAndFragment(t *testing.T) {
	got := RedactURL("https://user:password@example.com/api?code=abc#access_token=" + secretToken)
	if strings.Contains(got, "password") || strings.Contains(got, secretToken) || strings.Contains(got, "abc") {
		t.Fatalf("credentials survived: %q", got)
	}
}

func TestRedactURL_InvalidInputDoesNotEcho(t *testing.T) {
	got := RedactURL("://bad\x00url?access_token=" + secretToken)
	if strings.Contains(got, secretToken) {
		t.Fatalf("invalid URL input echoed: %q", got)
	}
}

func TestRedactURLError_ErrorStringHasNoTokenButKeepsCause(t *testing.T) {
	ue := &url.Error{
		Op:  "Get",
		URL: "https://graph.qq.com/u?access_token=" + secretToken + "&fmt=json",
		Err: errors.New("dial tcp: timeout"),
	}
	wrapped := fmt.Errorf("oauth: %w", redactURLError(ue))

	if strings.Contains(wrapped.Error(), secretToken) {
		t.Fatalf("token leaked through error chain: %q", wrapped.Error())
	}
	if !strings.Contains(wrapped.Error(), "dial tcp: timeout") {
		t.Fatalf("diagnostic cause was lost: %q", wrapped.Error())
	}
	var target *url.Error
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As(*url.Error) must still work after redaction")
	}
}

func TestRedactURLError_NonURLErrorUnchanged(t *testing.T) {
	original := errors.New("plain failure")
	if got := redactURLError(original); got != original {
		t.Fatalf("non-url error should be returned as-is, got %v", got)
	}
}
