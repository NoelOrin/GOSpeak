package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

const leakedAccessToken = "SECRET_ACCESS_TOKEN_9f3a"

// 回归 B2：OAUTH_GET_USER_FAILED 的 AppError.Message 可能带 userinfo URL，
// 而 QQ 把 access_token 放在 URL query 里。302 通道绝不能回显它。
func TestOAuthErrorMessage_NeverLeaksUpstreamDetail(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "get user failed with token in url.Error",
			err: pkg.NewAppError(pkg.OAUTH_GET_USER_FAILED,
				`Get "https://graph.qq.com/user/get_user_info?access_token=`+leakedAccessToken+`&fmt=json": dial tcp: timeout`),
			want: pkg.GetErrMsg(pkg.OAUTH_GET_USER_FAILED),
		},
		{
			name: "token exchange failed with token in url.Error",
			err: pkg.NewAppError(pkg.OAUTH_TOKEN_EXCHANGE_FAILED,
				`Post "https://example.com/token?client_secret=`+leakedAccessToken+`": EOF`),
			want: pkg.GetErrMsg(pkg.OAUTH_TOKEN_EXCHANGE_FAILED),
		},
		{
			name: "provider disabled custom message",
			err:  pkg.NewAppError(pkg.OAUTH_PROVIDER_DISABLED, "db row: "+leakedAccessToken),
			want: pkg.GetErrMsg(pkg.OAUTH_PROVIDER_DISABLED),
		},
		{
			name: "internal error falls back",
			err:  pkg.NewAppError(pkg.INTERNAL_ERROR, "sql: "+leakedAccessToken),
			want: "oauth login failed",
		},
		{
			name: "plain error falls back",
			err:  errors.New("url.Error: " + leakedAccessToken),
			want: "oauth login failed",
		},
		{
			name: "nil error falls back",
			err:  nil,
			want: "oauth login failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := oauthErrorMessage(tc.err, "oauth login failed")
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if strings.Contains(got, leakedAccessToken) {
				t.Fatalf("redirect message leaked token: %q", got)
			}
		})
	}
}

// 端到端覆盖 302 Location：错误文案经过清洗后不含 token，也不含控制字符。
func TestRedirectOAuthError_LocationNeverCarriesToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/cb", func(c *gin.Context) {
		err := pkg.NewAppError(pkg.OAUTH_GET_USER_FAILED,
			`Get "https://graph.qq.com/u?access_token=`+leakedAccessToken+`": timeout`)
		redirectOAuthError(c, oauthErrorMessage(err, "oauth login failed"))
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cb", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if strings.Contains(loc, leakedAccessToken) || strings.Contains(loc, "graph.qq.com") {
		t.Fatalf("Location leaked upstream detail: %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if u.Path != "/login" {
		t.Fatalf("unexpected redirect path %q", u.Path)
	}
	if got := u.Query().Get("oauth_error"); got != pkg.GetErrMsg(pkg.OAUTH_GET_USER_FAILED) {
		t.Fatalf("unexpected oauth_error %q", got)
	}
}

func TestRedirectOAuthError_SanitizesControlCharsAndLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/cb", func(c *gin.Context) {
		redirectOAuthError(c, "bad\nline\r\x00"+strings.Repeat("x", 500))
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cb", nil))

	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	msg := u.Query().Get("oauth_error")
	if strings.ContainsAny(msg, "\n\r\x00") {
		t.Fatalf("control characters survived: %q", msg)
	}
	if n := len([]rune(msg)); n != maxOAuthRedirectMessage {
		t.Fatalf("message not truncated to %d runes, got %d", maxOAuthRedirectMessage, n)
	}
}
