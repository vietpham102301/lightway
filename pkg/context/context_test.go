package context

import (
	"bytes"
	_context "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newContext(method, target string, body []byte) (*Context, *httptest.ResponseRecorder) {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	w := httptest.NewRecorder()
	return &Context{W: w, R: req}, w
}

// ===========================================================================
// JSONResponse
// ===========================================================================

func TestJSONResponse_Success(t *testing.T) {
	c, w := newContext("GET", "/", nil)

	c.JSONResponse(http.StatusOK, map[string]string{"key": "value"}, nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	var resp AppResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Errorf("expected response code %d, got %d", http.StatusOK, resp.Code)
	}
	if resp.Error != "" {
		t.Errorf("expected empty error, got %q", resp.Error)
	}
}

func TestJSONResponse_WithError(t *testing.T) {
	c, w := newContext("GET", "/", nil)

	c.JSONResponse(http.StatusBadRequest, nil, fmt.Errorf("invalid input"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp AppResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "invalid input" {
		t.Errorf("expected error 'invalid input', got %q", resp.Error)
	}
}

// ===========================================================================
// WriteErrorResponse
// ===========================================================================

func TestWriteErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorResponse(w, http.StatusForbidden, "access denied", nil)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var resp AppResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != http.StatusForbidden {
		t.Errorf("expected code %d, got %d", http.StatusForbidden, resp.Code)
	}
	if resp.Error != "access denied" {
		t.Errorf("expected error 'access denied', got %q", resp.Error)
	}
	if resp.Data != nil {
		t.Errorf("expected nil data, got %v", resp.Data)
	}
}

// ===========================================================================
// BindJSON
// ===========================================================================

func TestBindJSON(t *testing.T) {
	body := []byte(`{"name":"John","age":30}`)
	c, _ := newContext("POST", "/", body)

	var result struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := c.BindJSON(&result); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Name != "John" {
		t.Errorf("expected name 'John', got %q", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("expected age 30, got %d", result.Age)
	}
}

func TestBindJSON_InvalidJSON(t *testing.T) {
	body := []byte(`{invalid}`)
	c, _ := newContext("POST", "/", body)

	var result struct{}
	if err := c.BindJSON(&result); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ===========================================================================
// Query Parameters
// ===========================================================================

func TestQuery(t *testing.T) {
	c, _ := newContext("GET", "/test?search=hello&page=2", nil)

	if v := c.Query("search"); v != "hello" {
		t.Errorf("expected 'hello', got %q", v)
	}
	if v := c.Query("missing"); v != "" {
		t.Errorf("expected empty string for missing key, got %q", v)
	}
}

func TestQueryInt(t *testing.T) {
	c, _ := newContext("GET", "/test?page=5&invalid=abc", nil)

	if v := c.QueryInt("page", 1); v != 5 {
		t.Errorf("expected 5, got %d", v)
	}
	if v := c.QueryInt("missing", 10); v != 10 {
		t.Errorf("expected default 10 for missing key, got %d", v)
	}
	if v := c.QueryInt("invalid", 10); v != 10 {
		t.Errorf("expected default 10 for non-numeric value, got %d", v)
	}
}

// ===========================================================================
// Path Parameters (requires Go 1.22+ ServeMux routing)
// ===========================================================================

func TestParam_ViaServeMux(t *testing.T) {
	mux := http.NewServeMux()
	var capturedID string

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		c := &Context{W: w, R: r}
		capturedID = c.Param("id")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/users/abc123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if capturedID != "abc123" {
		t.Errorf("expected 'abc123', got %q", capturedID)
	}
}

func TestParamInt_ViaServeMux(t *testing.T) {
	mux := http.NewServeMux()
	var capturedID int
	var capturedErr error

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		c := &Context{W: w, R: r}
		capturedID, capturedErr = c.ParamInt("id")
		w.WriteHeader(http.StatusOK)
	})

	t.Run("Valid int param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/42", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if capturedErr != nil {
			t.Fatalf("expected no error, got %v", capturedErr)
		}
		if capturedID != 42 {
			t.Errorf("expected 42, got %d", capturedID)
		}
	})

	t.Run("Invalid int param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/abc", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if capturedErr == nil {
			t.Error("expected error for non-numeric param")
		}
	})
}

func TestParamInt_EmptyValue(t *testing.T) {
	// When called outside of a mux with no path value set, Param returns ""
	c, _ := newContext("GET", "/", nil)
	val, err := c.ParamInt("id")
	if err == nil {
		t.Error("expected error for empty param")
	}
	if val != -1 {
		t.Errorf("expected -1 for empty param, got %d", val)
	}
}

// ===========================================================================
// GetUserID
// ===========================================================================

func TestGetUserID_Success(t *testing.T) {
	c, _ := newContext("GET", "/", nil)
	ctx := withUserID(c.R.Context(), 99)
	c.R = c.R.WithContext(ctx)

	id, err := c.GetUserID()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != 99 {
		t.Errorf("expected 99, got %d", id)
	}
}

func TestGetUserID_NotSet(t *testing.T) {
	c, _ := newContext("GET", "/", nil)

	_, err := c.GetUserID()
	if err == nil {
		t.Error("expected error when user_id is not in context")
	}
}

func TestGetUserID_WrongType(t *testing.T) {
	c, _ := newContext("GET", "/", nil)
	ctx := withValue(c.R.Context(), UserIDKey, "not-an-int")
	c.R = c.R.WithContext(ctx)

	_, err := c.GetUserID()
	if err == nil {
		t.Error("expected error when user_id is wrong type")
	}
}

// ===========================================================================
// Status
// ===========================================================================

func TestStatus(t *testing.T) {
	c, w := newContext("GET", "/", nil)
	c.Status(http.StatusAccepted)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, w.Code)
	}
}

// ===========================================================================
// Helpers
// ===========================================================================

func withUserID(ctx _context.Context, id int) _context.Context {
	return _context.WithValue(ctx, UserIDKey, id)
}

func withValue(ctx _context.Context, key contextKey, val any) _context.Context {
	return _context.WithValue(ctx, key, val)
}
