package handler

import (
	"crypto/rand"
	"encoding/hex"
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
}

func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService}
}

func (h *OAuthHandler) Login(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		redirectOAuthError(c, "provider is required")
		return
	}

	// 服务端生成高熵 state，写入 HttpOnly cookie，callback 强制校验，防 OAuth Login CSRF。
	state := c.Query("state")
	if state == "" {
		generated, err := randomState(16)
		if err != nil {
			redirectOAuthError(c, "failed to generate oauth state")
			return
		}
		state = generated
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
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

	// 浏览器回调：把 token 交给登录页完成会话落地，避免 API JSON 卡在新页面。
	q := make(url.Values)
	q.Set("oauth", "1")
	q.Set("access_token", resp.Token)
	q.Set("refresh_token", resp.RefreshToken)
	c.Redirect(http.StatusFound, "/login#"+q.Encode())
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
