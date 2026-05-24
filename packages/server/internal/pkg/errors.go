package pkg

type ErrCode int

const (
	SUCCESS          ErrCode = 0
	ERROR            ErrCode = 1
	TOKEN_NOT_EXIST  ErrCode = 1001
	TOKEN_WRONG      ErrCode = 1002
	TOKEN_RUNTIME    ErrCode = 1003
	INVALID_PARAMS   ErrCode = 2001
	UNAUTHORIZED     ErrCode = 2002
	NOT_FOUND        ErrCode = 2003
	INTERNAL_ERROR   ErrCode = 5001
)

var errMsg = map[ErrCode]string{
	SUCCESS:        "success",
	ERROR:          "fail",
	TOKEN_NOT_EXIST: "token does not exist",
	TOKEN_WRONG:    "token is wrong",
	TOKEN_RUNTIME:  "token has expired",
	INVALID_PARAMS: "invalid parameters",
	UNAUTHORIZED:   "unauthorized",
	NOT_FOUND:      "resource not found",
	INTERNAL_ERROR: "internal server error",
}

func GetErrMsg(code ErrCode) string {
	if msg, ok := errMsg[code]; ok {
		return msg
	}
	return "unknown error"
}