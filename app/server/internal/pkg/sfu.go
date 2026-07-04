package pkg

import "errors"

// ErrSFUNotSupported 表示当前 SFU provider 不支持该操作。
// Hub 调用 RemoveParticipant / DeleteRoom 时用 errors.Is(err, ErrSFUNotSupported)
// 优雅降级，避免硬编码 provider 名判断。放 pkg 包以避开 sfu <-> provider 的 import 循环。
var ErrSFUNotSupported = errors.New("sfu: operation not supported by provider")

// NewErrSFUNotSupported 返回携带 ErrSFUNotSupported cause 的 AppError，
// 供各 SFU provider 复用：既保留 SFU_ERROR 状态码供 HTTP 层使用，又使 errors.Is(err, ErrSFUNotSupported) 成立。
func NewErrSFUNotSupported() *AppError {
	return NewAppErrorWithCause(SFU_ERROR, ErrSFUNotSupported, ErrSFUNotSupported.Error())
}
