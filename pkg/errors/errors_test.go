package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	appErr := NewAppError(http.StatusBadRequest, "something went wrong", nil)
	if appErr.Error() != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", appErr.Error())
	}
}

func TestAppError_ImplementsErrorInterface(t *testing.T) {
	var err error = NewAppError(http.StatusBadRequest, "test", nil)
	if err == nil {
		t.Error("expected non-nil error")
	}
}

func TestAppError_ErrorsAs(t *testing.T) {
	original := NewAppError(http.StatusNotFound, "not found", nil)
	wrapped := fmt.Errorf("handler failed: %w", original)

	var appErr *AppError
	if !errors.As(wrapped, &appErr) {
		t.Fatal("errors.As should unwrap to *AppError")
	}
	if appErr.Code != http.StatusNotFound {
		t.Errorf("expected code %d, got %d", http.StatusNotFound, appErr.Code)
	}
	if appErr.Message != "not found" {
		t.Errorf("expected message 'not found', got %q", appErr.Message)
	}
}

func TestNewAppError(t *testing.T) {
	inner := fmt.Errorf("db connection failed")
	appErr := NewAppError(http.StatusInternalServerError, "Internal Error", inner)

	if appErr.Code != http.StatusInternalServerError {
		t.Errorf("expected code %d, got %d", http.StatusInternalServerError, appErr.Code)
	}
	if appErr.Message != "Internal Error" {
		t.Errorf("expected message 'Internal Error', got %q", appErr.Message)
	}
	if appErr.Err != inner {
		t.Error("expected inner error to be preserved")
	}
}

func TestInvalidRequest(t *testing.T) {
	inner := fmt.Errorf("missing field")
	appErr := InvalidRequest(inner)

	if appErr.Code != http.StatusBadRequest {
		t.Errorf("expected code %d, got %d", http.StatusBadRequest, appErr.Code)
	}
	if appErr.Message != "Invalid Request" {
		t.Errorf("expected message 'Invalid Request', got %q", appErr.Message)
	}
	if appErr.Err != inner {
		t.Error("expected inner error to be preserved")
	}
}

func TestNotFound(t *testing.T) {
	appErr := NotFound("user not found")

	if appErr.Code != http.StatusNotFound {
		t.Errorf("expected code %d, got %d", http.StatusNotFound, appErr.Code)
	}
	if appErr.Message != "user not found" {
		t.Errorf("expected message 'user not found', got %q", appErr.Message)
	}
	if appErr.Err != nil {
		t.Error("expected nil inner error for NotFound")
	}
}

func TestUnauthorized(t *testing.T) {
	appErr := Unauthorized("invalid token")

	if appErr.Code != http.StatusUnauthorized {
		t.Errorf("expected code %d, got %d", http.StatusUnauthorized, appErr.Code)
	}
	if appErr.Message != "invalid token" {
		t.Errorf("expected message 'invalid token', got %q", appErr.Message)
	}
}

func TestInternalServerError(t *testing.T) {
	appErr := InternalServerError()

	if appErr.Code != http.StatusInternalServerError {
		t.Errorf("expected code %d, got %d", http.StatusInternalServerError, appErr.Code)
	}
	if appErr.Message != "Internal Server Error" {
		t.Errorf("expected message 'Internal Server Error', got %q", appErr.Message)
	}
}
