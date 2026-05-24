package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func JwtToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenHeader := c.Request.Header.Get("Authorization")
		code := utils.SUCCESS
		if tokenHeader == "" {
			code = utils.TOKEN_NOT_EXIST
			c.JSON(http.StatusOK, serializer.CheckToken(
				code,
				utils.GetErrMsg(code)))
			c.Abort()
			return
		}

		key, tCode := CheckToken(tokenHeader)
		if tCode == utils.ERROR {
			code = utils.TOKEN_WRONG
			c.JSON(http.StatusOK, serializer.CheckToken(
				code,
				utils.GetErrMsg(code)))
			c.Abort()
			return
		}

		//判断token是否过期
		if time.Now().Unix() > key.ExpiresAt.Unix() {
			code = utils.TOKEN_RUNTIME
			c.JSON(http.StatusOK, serializer.CheckToken(
				code,
				utils.GetErrMsg(code)))
			c.Abort()
			return
		}

		c.Set("username", key.Username)
		c.Next()
	}
}
