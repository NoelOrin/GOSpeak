package pkg

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Response struct {
	Code    ErrCode     `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    SUCCESS,
		Message: GetErrMsg(SUCCESS),
		Data:    data,
	})
}

func Fail(c *gin.Context, code ErrCode) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: GetErrMsg(code),
	})
}

func FailWithMsg(c *gin.Context, code ErrCode, msg string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: msg,
	})
}

func Error(c *gin.Context, httpStatus int, code ErrCode) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: GetErrMsg(code),
	})
}

func ErrorWithMsg(c *gin.Context, httpStatus int, code ErrCode, msg string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: msg,
	})
}