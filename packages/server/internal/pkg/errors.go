package pkg

// ErrCode 业务状态码
// 0         = 成功
// 1xxx      = 认证相关错误
// 2xxx      = 参数校验错误
// 3xxx      = 资源相关错误
// 5xxx      = 服务端内部错误
// 6xxx      = LiveKit 相关错误
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

	// Parameter errors 2xxx
	INVALID_PARAMS ErrCode = 2001

	// Resource errors 3xxx
	NOT_FOUND      ErrCode = 3001
	ALREADY_EXISTS ErrCode = 3002

	// Server errors 5xxx
	INTERNAL_ERROR ErrCode = 5001

	// LiveKit errors 6xxx
	LIVEKIT_NOT_CONFIGURED ErrCode = 6001
	LIVEKIT_ERROR          ErrCode = 6002
)

var errMsg = map[ErrCode]string{
	SUCCESS:                "success",
	TOKEN_NOT_EXIST:        "token does not exist",
	TOKEN_WRONG:            "token is wrong",
	TOKEN_EXPIRED:          "token has expired",
	INVALID_PASSWORD:       "invalid password",
	USER_NOT_FOUND:         "user not found",
	USERNAME_EXISTS:        "username already exists",
	INVALID_PARAMS:         "invalid parameters",
	NOT_FOUND:              "resource not found",
	ALREADY_EXISTS:         "resource already exists",
	INTERNAL_ERROR:         "internal server error",
	LIVEKIT_NOT_CONFIGURED: "livekit not configured",
	LIVEKIT_ERROR:          "livekit error",
}

func GetErrMsg(code ErrCode) string {
	if msg, ok := errMsg[code]; ok {
		return msg
	}
	return "unknown error"
}

// AppError 业务错误，携带状态码，可在 service 层直接返回
type AppError struct {
	Code    ErrCode
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code ErrCode, msg ...string) *AppError {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	return &AppError{Code: code, Message: m}
}