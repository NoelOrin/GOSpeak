package router

import (
	"github.com/gin-gonic/gin"
	// "go_rtc/server/service"
)

func UserRoute(r *gin.Engine) *gin.Engine {

	{
		v1 := r.Group("/user/api/v1")
		v1.POST("/login", func(ctx *gin.Context) {

		})
		v1.POST("/logout", func(ctx *gin.Context) {

		})
		// 注册
		v1.POST("/sign_up", func(ctx *gin.Context) {

		})
		v1.POST("/get_refresh_token", func(ctx *gin.Context) {

		})
		v1.POST("/get_refresh_token", func(ctx *gin.Context) {

		})
	}

	return r
}
