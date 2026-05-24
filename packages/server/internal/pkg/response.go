package pkg

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应体
//   成功时: { "code": 0,     "msg": "success",          "data": {...} }
//   失败时: { "code": 1001,  "msg": "token does not exist", "data": null }
type Response struct {
	Code ErrCode     `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: SUCCESS,
		Msg:  GetErrMsg(SUCCESS),
		Data: data,
	})
}

// Fail 返回失败响应，通过 AppError 或 ErrCode 指定错误码
func Fail(c *gin.Context, code ErrCode, msg ...string) {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  m,
		Data: nil,
	})
}

// HandleError 统一处理 service 层返回的错误
//   如果 err 是 *AppError，提取 Code 和 Message；
//   否则使用 INTERNAL_ERROR 兜底
func HandleError(c *gin.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		Fail(c, appErr.Code, appErr.Message)
		return
	}
	Fail(c, INTERNAL_ERROR)
}