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

// Fail 返回失败响应，根据业务错误码自动映射 HTTP 状态码
func Fail(c *gin.Context, code ErrCode, msg ...string) {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	c.JSON(errToHTTPStatus(code), Response{
		Code: code,
		Msg:  m,
		Data: nil,
	})
}

// errToHTTPStatus 将业务错误码映射为 HTTP 状态码
func errToHTTPStatus(code ErrCode) int {
	switch {
	case code == FORBIDDEN || code == USER_BANNED:
		return http.StatusForbidden // 403
	case code >= 1001 && code <= 1015:
		return http.StatusUnauthorized // 401
	case code >= 2001 && code <= 2999:
		return http.StatusBadRequest // 400
	case code == NOT_FOUND:
		return http.StatusNotFound // 404
	case code >= 5001 && code <= 5999:
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError // 500
	}
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