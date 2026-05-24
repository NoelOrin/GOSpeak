package router

import (
	"github.com/gin-gonic/gin"
)

func IndexRoute(r *gin.Engine) *gin.Engine {
	{
		v1 := r.Group("/sign")
		v1.POST("/login", func(context *gin.Context) {

		})
		v1.POST("/submit", func(context *gin.Context) {

		})
		v1.POST("/read", func(context *gin.Context) {

		})
	}

	return r
}
