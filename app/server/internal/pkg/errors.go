package pkg

// ErrCode 业务状态码
// 0         = 成功
// 1xxx      = 认证相关错误
// 2xxx      = 参数校验错误
// 3xxx      = 资源相关错误
// 5xxx      = 服务端内部错误
// 6xxx      = SFU 相关错误
type ErrCode int

const (
	// Success
	SUCCESS ErrCode = 0

	// Auth errors 1xxx
	TOKEN_NOT_EXIST  ErrCode = 1001
	TOKEN_WRONG      ErrCode = 1002
	TOKEN_EXPIRED    ErrCode = 1003
	INVALID_PASSWORD ErrCode = 1010
	USER_NOT_FOUND   ErrCode = 1011
	USERNAME_EXISTS  ErrCode = 1012
	FORBIDDEN        ErrCode = 1013
	TOKEN_REVOKED    ErrCode = 1014
	USER_BANNED      ErrCode = 1015
	USER_MUTED       ErrCode = 1016

	// Parameter errors 2xxx
	INVALID_PARAMS ErrCode = 2001

	// Resource errors 3xxx
	NOT_FOUND      ErrCode = 3001
	ALREADY_EXISTS ErrCode = 3002

	// Server errors 5xxx
	INTERNAL_ERROR ErrCode = 5001

	// SFU errors 6xxx
	SFU_NOT_CONFIGURED ErrCode = 6001
	SFU_ERROR          ErrCode = 6002

	// OAuth errors 7xxx
	OAUTH_PROVIDER_NOT_FOUND    ErrCode = 7001
	OAUTH_PROVIDER_DISABLED     ErrCode = 7002
	OAUTH_TOKEN_EXCHANGE_FAILED ErrCode = 7003
	OAUTH_GET_USER_FAILED       ErrCode = 7004

	// Email errors 8xxx
	EMAIL_ALREADY_EXISTS    ErrCode = 8001
	EMAIL_CODE_INVALID      ErrCode = 8002
	EMAIL_CODE_EXPIRED      ErrCode = 8003
	EMAIL_CODE_ALREADY_USED ErrCode = 8004
	EMAIL_SEND_TOO_FREQUENT ErrCode = 8005
	EMAIL_SEND_FAILED       ErrCode = 8006
	EMAIL_NOT_VERIFIED      ErrCode = 8007
	EMAIL_CODE_MAX_ATTEMPTS ErrCode = 8008
	EMAIL_NOT_CONFIGURED    ErrCode = 8009
	PASSWORD_RESET_DISABLED ErrCode = 8010

	// Storage errors 81xx
	STORAGE_NOT_CONFIGURED        ErrCode = 8101
	STORAGE_ERROR                 ErrCode = 8102
	STORAGE_FILE_TOO_LARGE        ErrCode = 8103
	STORAGE_FILE_TYPE_NOT_ALLOWED ErrCode = 8104
)

var errMsg = map[ErrCode]string{
	SUCCESS:            "success",
	TOKEN_NOT_EXIST:    "token does not exist",
	TOKEN_WRONG:        "token is wrong",
	TOKEN_EXPIRED:      "token has expired",
	TOKEN_REVOKED:      "token has been revoked",
	INVALID_PASSWORD:   "invalid password",
	USER_NOT_FOUND:     "user not found",
	USERNAME_EXISTS:    "username already exists",
	FORBIDDEN:          "forbidden",
	USER_BANNED:        "user has been banned",
	USER_MUTED:         "user has been muted",
	INVALID_PARAMS:     "invalid parameters",
	NOT_FOUND:          "resource not found",
	ALREADY_EXISTS:     "resource already exists",
	INTERNAL_ERROR:     "internal server error",
	SFU_NOT_CONFIGURED: "sfu not configured",
	SFU_ERROR:          "sfu error",

	OAUTH_PROVIDER_NOT_FOUND:      "oauth provider not found",
	OAUTH_PROVIDER_DISABLED:       "oauth provider is disabled",
	OAUTH_TOKEN_EXCHANGE_FAILED:   "oauth token exchange failed",
	OAUTH_GET_USER_FAILED:         "oauth get user info failed",
	EMAIL_ALREADY_EXISTS:          "email already exists",
	EMAIL_CODE_INVALID:            "verification code is invalid",
	EMAIL_CODE_EXPIRED:            "verification code expired",
	EMAIL_CODE_ALREADY_USED:       "verification code already used",
	EMAIL_SEND_TOO_FREQUENT:       "send too frequent",
	EMAIL_SEND_FAILED:             "email send failed",
	EMAIL_NOT_VERIFIED:            "email not verified",
	EMAIL_CODE_MAX_ATTEMPTS:       "max verification attempts exceeded",
	EMAIL_NOT_CONFIGURED:          "email verification is not configured",
	PASSWORD_RESET_DISABLED:       "password reset is disabled",
	STORAGE_NOT_CONFIGURED:        "storage not configured",
	STORAGE_ERROR:                 "storage error",
	STORAGE_FILE_TOO_LARGE:        "storage file too large",
	STORAGE_FILE_TYPE_NOT_ALLOWED: "storage file type not allowed",
}

func GetErrMsg(code ErrCode) string {
	if msg, ok := errMsg[code]; ok {
		return msg
	}
	return "unknown error"
}

// String 返回错误码对应的默认消息，便于日志与 WS 鉴权失败原因输出。
func (c ErrCode) String() string {
	return GetErrMsg(c)
}

// AppError 业务错误，携带状态码，可在 service 层直接返回
type AppError struct {
	Code    ErrCode
	Message string
	// Cause 可选的底层错误；设置后 errors.Is/As 可沿此链匹配 sentinel error。
	Cause error `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

// Unwrap 暴露 Cause，使 errors.Is(err, sentinel) 能穿透 AppError 匹配到底层 sentinel。
func (e *AppError) Unwrap() error {
	return e.Cause
}

func NewAppError(code ErrCode, msg ...string) *AppError {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	return &AppError{Code: code, Message: m}
}

// NewAppErrorWithCause 构造携带底层 cause 的 AppError。
// 既保留 Code/Message 供 HTTP 响应层使用，又让 errors.Is(err, cause) 成立。
func NewAppErrorWithCause(code ErrCode, cause error, msg ...string) *AppError {
	e := NewAppError(code, msg...)
	e.Cause = cause
	return e
}
