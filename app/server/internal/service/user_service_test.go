package service

import (
	"errors"
	"testing"

	"GOSpeak/internal/pkg"
)

// assertAppErrorCode 断言业务错误码，供本包多个 service 测试复用。
func assertAppErrorCode(t *testing.T, err error, want pkg.ErrCode) {
	t.Helper()
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("expected code %d, got %d: %v", want, appErr.Code, err)
	}
}
