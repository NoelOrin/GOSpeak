package pkg

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func performHandleError(err error) (int, Response) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	HandleError(c, err)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		panic(err)
	}
	return w.Code, resp
}

func TestHandleError_HidesInternalDetails(t *testing.T) {
	t.Parallel()
	code, resp := performHandleError(NewAppError(INTERNAL_ERROR, "sql: connection refused"))
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if resp.Code != INTERNAL_ERROR {
		t.Fatalf("code = %d, want INTERNAL_ERROR", resp.Code)
	}
	if resp.Msg != GetErrMsg(INTERNAL_ERROR) {
		t.Fatalf("msg = %q, want default internal message", resp.Msg)
	}
}

func TestHandleError_KeepsBusinessMessage(t *testing.T) {
	t.Parallel()
	code, resp := performHandleError(NewAppError(INVALID_PARAMS, "provider is required"))
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
	if resp.Msg != "provider is required" {
		t.Fatalf("msg = %q, want business message", resp.Msg)
	}
}

func TestHandleError_PlainErrorUsesDefault(t *testing.T) {
	t.Parallel()
	code, resp := performHandleError(errors.New("boom"))
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if resp.Msg != GetErrMsg(INTERNAL_ERROR) {
		t.Fatalf("msg = %q, want default internal message", resp.Msg)
	}
}

func TestHandleError_LoginErrorsUse401(t *testing.T) {
	t.Parallel()
	for _, code := range []ErrCode{USER_NOT_FOUND, INVALID_PASSWORD} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
		HandleError(c, NewAppError(code, "invalid credentials"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("code %d status = %d, want 401", code, w.Code)
		}
	}
}

func TestHandleError_RegisterErrorsUse400(t *testing.T) {
	t.Parallel()
	for _, code := range []ErrCode{USERNAME_EXISTS, EMAIL_ALREADY_EXISTS} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/register", nil)
		HandleError(c, NewAppError(code, "duplicate"))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("code %d status = %d, want 400", code, w.Code)
		}
	}
}
