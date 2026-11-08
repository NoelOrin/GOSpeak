package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

const oauthStateCookie = "gospeak_oauth_state"

type OAuthHandler struct {
	oauthService *service.OAuthService
	cookie       *AuthCookieConfig
}

func NewOAuthHandler(oauthService *service.OAuthService, cookie *AuthCookieConfig) *OAuthHandler {
	if cookie == nil {
		cookie = defaultAuthCookieConfig()
	}
	return &OAuthHandler{oauthService: oauthService, cookie: cookie}
}

func (h *OAuthHandler) Login(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		redirectOAuthError(c, "provider is required")
		return
	}

	// 服务端生成高熵 state，写入 HttpOnly cookie，callback 强制校验，防 OAuth Login CSRF。
	// 不信任客户端传入的 state：统一服务端生成，避免预置固定 state。
	generated, err := randomState(16)
	if err != nil {
		redirectOAuthError(c, "failed to generate oauth state")
		return
	}
	state := generated
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.cookie.secure(c.Request),
		SameSite: http.SameSiteLaxMode,
	})

	authURL, err := h.oauthService.GetAuthURL(provider, state)
	if err != nil {
		// 浏览器从登录页跳入，错误统一回登录页展示，避免卡在 JSON 页。
		redirectOAuthError(c, oauthErrorMessage(err, "oauth provider unavailable"))
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

func (h *OAuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	if code == "" {
		redirectOAuthError(c, "code is required")
		return
	}

	state := c.Query("state")
	cookie, err := c.Request.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || state == "" || state != cookie.Value {
		redirectOAuthError(c, "invalid oauth state")
		return
	}
	// 一次性 state
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.secure(c.Request),
		SameSite: http.SameSiteLaxMode,
	})

	// 授权方错误回传（用户拒绝等）
	if errDesc := c.Query("error"); errDesc != "" {
		msg := errDesc
		if d := c.Query("error_description"); d != "" {
			msg = d
		}
		redirectOAuthError(c, msg)
		return
	}

	resp, err := h.oauthService.HandleCallback(provider, code)
	if err != nil {
		redirectOAuthError(c, oauthErrorMessage(err, "oauth login failed"))
		return
	}

	h.cookie.Set(c, resp.Token, resp.RefreshToken)

	// 浏览器回调：token 已写入 HttpOnly Cookie，同源 postMessage 只通知 opener 成功，
	// 避免 token 出现在 URL fragment、浏览器历史、代理日志或跨窗口消息中。
	payload, _ := oauthSuccessPayload()
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'")
	_, _ = c.Writer.Write([]byte(oauthBridgeHTML(payload)))
}

// oauthSuccessPayload 通知 opener 登录成功并附带 access token 有效期（秒）。
func oauthSuccessPayload() ([]byte, error) {
	return json.Marshal(map[string]any{"ok": true, "expires_in": pkg.AccessTokenExpiresIn()})
}

// oauthBridgeHTML 返回 OAuth 回调落地页：同源窗口通过 postMessage 单次交付 token。
func oauthBridgeHTML(payload []byte) string {
	return `<!doctype html>
<html lang="zh-CN">
<meta charset="utf-8">
<title>GOSpeak 登录</title>
<body>
<script>
(function () {
  var data = ` + string(payload) + `;
  if (window.opener && window.opener !== window) {
    try {
      window.opener.postMessage({type: "gospeak-oauth", ok: data.ok === true}, window.location.origin);
    } catch (e) {}
    window.close();
    return;
  }
  document.body.textContent = "登录成功，请关闭本窗口返回 GOSpeak。";
})();
</script>
</body>
</html>`
}

func redirectOAuthError(c *gin.Context, msg string) {
	q := make(url.Values)
	q.Set("oauth_error", msg)
	c.Redirect(http.StatusFound, "/login?"+q.Encode())
}

func oauthErrorMessage(err error, fallback string) string {
	var appErr *pkg.AppError
	if errors.As(err, &appErr) {
		if appErr.Message != "" {
			return appErr.Message
		}
		if msg := pkg.GetErrMsg(appErr.Code); msg != "" {
			return msg
		}
	}
	return fallback
}

func randomState(n int) (string, error) {
	b := make([]byte, n)
	for i := 0; i < 3; i++ {
		if _, err := rand.Read(b); err == nil {
			return hex.EncodeToString(b), nil
		}
	}
	return "", errors.New("oauth state: crypto/rand unavailable")
}

func (h *OAuthHandler) ListEnabledProviders(c *gin.Context) {
	providers, err := h.oauthService.ListEnabledProviders()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, providers)
}

func (h *OAuthHandler) ListProviders(c *gin.Context) {
	providers, err := h.oauthService.ListProviders()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, providers)
}

func (h *OAuthHandler) CreateProvider(c *gin.Context) {
	var req service.CreateOAuthProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.Name == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "name is required")
		return
	}
	provider, err := h.oauthService.CreateProviderFromDTO(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, provider)
}

func (h *OAuthHandler) UpdateProvider(c *gin.Context) {
	var req service.UpdateOAuthProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	provider, err := h.oauthService.UpdateProviderFromDTO(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, provider)
}

func (h *OAuthHandler) DeleteProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}
	if err := h.oauthService.DeleteProvider(uint(id)); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
