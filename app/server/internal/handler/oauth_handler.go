package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type OAuthHandler struct {
	oauthService *service.OAuthService
}

func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService}
}

func (h *OAuthHandler) Login(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "provider is required")
		return
	}

	state := c.Query("state")
	if state == "" {
		state = "random_state"
	}

	authURL, err := h.oauthService.GetAuthURL(provider, state)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	c.Redirect(302, authURL)
}

func (h *OAuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	if code == "" {
		pkg.Fail(c, pkg.INVALID_PARAMS, "code is required")
		return
	}

	resp, err := h.oauthService.HandleCallback(provider, code)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, resp)
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
