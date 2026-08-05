package pkg

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应体
//
//	成功时: { "code": 0,     "msg": "success",          "data": {...} }
//	失败时: { "code": 1001,  "msg": "token does not exist", "data": null }
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
	switch code {
	case SUCCESS:
		return http.StatusOK
	case TOKEN_NOT_EXIST, TOKEN_WRONG, TOKEN_EXPIRED, TOKEN_REVOKED:
		return http.StatusUnauthorized
	case INVALID_PASSWORD, USER_NOT_FOUND, USERNAME_EXISTS, EMAIL_ALREADY_EXISTS:
		// 登录/注册业务错误：避免与 token 鉴权 401 混淆
		return http.StatusBadRequest
	case FORBIDDEN, USER_BANNED, USER_MUTED, PASSWORD_RESET_DISABLED:
		return http.StatusForbidden
	case RATE_LIMITED:
		return http.StatusTooManyRequests
	case INVALID_PARAMS, EMAIL_CODE_INVALID, EMAIL_CODE_EXPIRED, EMAIL_CODE_ALREADY_USED,
		EMAIL_SEND_TOO_FREQUENT, EMAIL_CODE_MAX_ATTEMPTS, EMAIL_NOT_VERIFIED,
		STORAGE_FILE_TOO_LARGE, STORAGE_FILE_TYPE_NOT_ALLOWED:
		return http.StatusBadRequest
	case NOT_FOUND, OAUTH_PROVIDER_NOT_FOUND:
		return http.StatusNotFound
	case ALREADY_EXISTS:
		return http.StatusConflict
	case SFU_NOT_CONFIGURED, EMAIL_NOT_CONFIGURED, STORAGE_NOT_CONFIGURED, OAUTH_PROVIDER_DISABLED:
		return http.StatusServiceUnavailable
	case OAUTH_TOKEN_EXCHANGE_FAILED, OAUTH_GET_USER_FAILED, SFU_ERROR, STORAGE_ERROR, EMAIL_SEND_FAILED:
		return http.StatusBadGateway
	case INTERNAL_ERROR:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// HandleError 统一处理 service 层返回的错误。
//
//	*AppError：业务码按需返回 Message；内部/上游错误仅返回默认文案，避免泄露实现细节。
//	其他 error：统一 INTERNAL_ERROR 兜底，不透传 err.Error()。
func HandleError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		if shouldHideErrorDetail(appErr.Code) {
			Fail(c, appErr.Code)
			return
		}
		Fail(c, appErr.Code, appErr.Message)
		return
	}
	Fail(c, INTERNAL_ERROR)
}

// ClientError 将服务层错误转换为客户端安全可见的（业务码，文案）。
// 与 HandleError 使用同一脱敏规则，供 WebSocket ACK 等非 HTTP 通道复用。
func ClientError(err error) (ErrCode, string) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		if shouldHideErrorDetail(appErr.Code) {
			return appErr.Code, GetErrMsg(appErr.Code)
		}
		return appErr.Code, appErr.Message
	}
	return INTERNAL_ERROR, GetErrMsg(INTERNAL_ERROR)
}

// shouldHideErrorDetail 判断该业务码是否可能携带底层实现细节，客户端只应看到默认文案。
func shouldHideErrorDetail(code ErrCode) bool {
	switch code {
	case INTERNAL_ERROR, SFU_ERROR, STORAGE_ERROR,
		OAUTH_TOKEN_EXCHANGE_FAILED, OAUTH_GET_USER_FAILED, EMAIL_SEND_FAILED:
		return true
	default:
		return false
	}
}
