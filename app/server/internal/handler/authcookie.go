package handler

import (
	"net/http"
	"strings"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// AuthCookieConfig 控制 access/refresh token 的 HttpOnly Cookie 属性。
type AuthCookieConfig struct {
	AccessName  string
	RefreshName string
	Domain      string
	Path        string
	Secure      string // auto|true|false
	SameSite    string // lax|strict|none
}

// NewAuthCookieConfig 从服务配置构造 Cookie 设置。
func NewAuthCookieConfig(cfg *config.Config) *AuthCookieConfig {
	if cfg == nil {
		cfg = &config.Config{}
	}
	c := &AuthCookieConfig{
		AccessName:  strings.TrimSpace(cfg.AccessCookieName),
		RefreshName: strings.TrimSpace(cfg.RefreshCookieName),
		Domain:      strings.TrimSpace(cfg.CookieDomain),
		Path:        strings.TrimSpace(cfg.CookiePath),
		Secure:      strings.ToLower(strings.TrimSpace(cfg.CookieSecure)),
		SameSite:    strings.ToLower(strings.TrimSpace(cfg.CookieSameSite)),
	}
	if c.AccessName == "" {
		c.AccessName = "gospeak_token"
	}
	if c.RefreshName == "" {
		c.RefreshName = "gospeak_refresh_token"
	}
	if c.Path == "" {
		c.Path = "/"
	}
	if c.Secure == "" {
		c.Secure = "auto"
	}
	if c.SameSite == "" {
		c.SameSite = "lax"
	}
	return c
}

// defaultAuthCookieConfig 供测试或未注入配置的路径使用。
func defaultAuthCookieConfig() *AuthCookieConfig {
	return NewAuthCookieConfig(&config.Config{})
}

// Set 将 access/refresh token 写入 HttpOnly Cookie。
func (c *AuthCookieConfig) Set(g *gin.Context, access, refresh string) {
	if access != "" {
		http.SetCookie(g.Writer, c.cookie(g, c.AccessName, access, int(pkg.AccessTokenTTL/time.Second)))
	}
	if refresh != "" {
		http.SetCookie(g.Writer, c.cookie(g, c.RefreshName, refresh, int(pkg.RefreshTokenTTL/time.Second)))
	}
}

// Clear 删除 access/refresh Cookie。
func (c *AuthCookieConfig) Clear(g *gin.Context) {
	http.SetCookie(g.Writer, c.cookie(g, c.AccessName, "", -1))
	http.SetCookie(g.Writer, c.cookie(g, c.RefreshName, "", -1))
}

func (c *AuthCookieConfig) cookie(g *gin.Context, name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     c.Path,
		Domain:   c.Domain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.secure(g.Request),
		SameSite: c.sameSite(),
	}
}

func (c *AuthCookieConfig) secure(r *http.Request) bool {
	switch c.Secure {
	case "true":
		return true
	case "false":
		return false
	default:
		return r.TLS != nil
	}
}

func (c *AuthCookieConfig) sameSite() http.SameSite {
	switch c.SameSite {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
