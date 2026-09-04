package swagger

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const initializerCN = `
window.onload = function() {
  var loc = window.location;
  var basePath = loc.pathname.split('/').slice(0, -1).join('/');
  var redirectUrl = loc.protocol + "//" + loc.host + basePath + "/oauth2-redirect.html";
  const ui = SwaggerUIBundle({
    url: "doc.json",
    dom_id: '#swagger-ui',
    validatorUrl: null,
    oauth2RedirectUrl: redirectUrl,
    persistAuthorization: false,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout",
    docExpansion: "list",
    deepLinking: true,
    defaultModelsExpandDepth: 1,
    locale: "zh-CN"
  })
  window.ui = ui
}
`

func Register(r *gin.Engine) {
	r.GET("/swagger/*any", func(c *gin.Context) {
		if c.Param("any") == "/swagger-initializer.js" {
			c.Header("Content-Type", "application/javascript")
			c.String(200, initializerCN)
			return
		}
		ginSwagger.WrapHandler(swaggerFiles.Handler)(c)
	})
}
