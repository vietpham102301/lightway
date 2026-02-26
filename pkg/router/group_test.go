package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vietpham102301/lightway/pkg/context"
	aerror "github.com/vietpham102301/lightway/pkg/errors"
)

// ===========================================================================
// Basic Routing
// ===========================================================================

func TestRouter_GET(t *testing.T) {
	r := NewRouter()
	r.GET("/hello", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("hello"))
		return nil
	})

	req := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got %q", w.Body.String())
	}
}

func TestRouter_POST(t *testing.T) {
	r := NewRouter()
	r.POST("/create", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusCreated)
		c.W.Write([]byte("created"))
		return nil
	})

	req := httptest.NewRequest("POST", "/create", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

func TestRouter_PUT(t *testing.T) {
	r := NewRouter()
	r.PUT("/update", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("updated"))
		return nil
	})

	req := httptest.NewRequest("PUT", "/update", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRouter_DELETE(t *testing.T) {
	r := NewRouter()
	r.DELETE("/remove", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusNoContent)
		return nil
	})

	req := httptest.NewRequest("DELETE", "/remove", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

// ===========================================================================
// Path Parameters
// ===========================================================================

func TestRouter_PathParams(t *testing.T) {
	r := NewRouter()
	r.GET("/users/{id}", func(c *context.Context) error {
		id := c.Param("id")
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte(id))
		return nil
	})

	req := httptest.NewRequest("GET", "/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "42" {
		t.Errorf("expected body '42', got %q", w.Body.String())
	}
}

func TestRouter_MultipleParams(t *testing.T) {
	r := NewRouter()
	r.GET("/users/{userId}/posts/{postId}", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte(c.Param("userId") + ":" + c.Param("postId")))
		return nil
	})

	req := httptest.NewRequest("GET", "/users/1/posts/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "1:99" {
		t.Errorf("expected body '1:99', got %q", w.Body.String())
	}
}

// ===========================================================================
// Route Groups
// ===========================================================================

func TestRouter_Group(t *testing.T) {
	r := NewRouter()
	api := r.Group("/api")
	v1 := api.Group("/v1")

	v1.GET("/users", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("users"))
		return nil
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "users" {
		t.Errorf("expected body 'users', got %q", w.Body.String())
	}
}

// ===========================================================================
// Middleware
// ===========================================================================

func TestRouter_Middleware(t *testing.T) {
	r := NewRouter()

	// Middleware that adds a header
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "applied")
			next.ServeHTTP(w, r)
		})
	})

	r.GET("/test", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Middleware") != "applied" {
		t.Error("expected middleware header to be set")
	}
}

func TestRouter_GroupMiddleware_Isolation(t *testing.T) {
	r := NewRouter()

	admin := r.Group("/admin")
	admin.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Admin", "true")
			next.ServeHTTP(w, r)
		})
	})
	admin.GET("/dashboard", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		return nil
	})

	r.GET("/public", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		return nil
	})

	// Admin route should have the header
	req := httptest.NewRequest("GET", "/admin/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Admin") != "true" {
		t.Error("expected admin middleware on /admin/dashboard")
	}

	// Public route should NOT have the header
	req = httptest.NewRequest("GET", "/public", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Admin") != "" {
		t.Error("expected no admin middleware on /public")
	}
}

func TestRouter_MiddlewareOrder(t *testing.T) {
	r := NewRouter()
	var order []string

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "first")
			next.ServeHTTP(w, r)
		})
	})
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "second")
			next.ServeHTTP(w, r)
		})
	})

	r.GET("/test", func(c *context.Context) error {
		order = append(order, "handler")
		c.W.WriteHeader(http.StatusOK)
		return nil
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := []string{"first", "second", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

// ===========================================================================
// WithMiddleware
// ===========================================================================

func TestRouter_WithMiddleware(t *testing.T) {
	r := NewRouter()
	r.GET("/test", func(c *context.Context) error {
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("ok"))
		return nil
	})

	handler := r.WithMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Outer", "yes")
			next.ServeHTTP(w, r)
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Outer") != "yes" {
		t.Error("expected WithMiddleware to apply outer middleware")
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", w.Body.String())
	}
}

// ===========================================================================
// Error Handling
// ===========================================================================

func TestRouter_AppErrorResponse(t *testing.T) {
	r := NewRouter()
	r.GET("/fail", func(c *context.Context) error {
		return aerror.NotFound("user not found")
	})

	req := httptest.NewRequest("GET", "/fail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "user not found" {
		t.Errorf("expected error 'user not found', got %v", resp["error"])
	}
}

func TestRouter_GenericErrorResponse(t *testing.T) {
	r := NewRouter()
	r.GET("/fail", func(c *context.Context) error {
		return fmt.Errorf("something unexpected")
	})

	req := httptest.NewRequest("GET", "/fail", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestRouter_NilErrorNoResponse(t *testing.T) {
	r := NewRouter()
	r.GET("/ok", func(c *context.Context) error {
		c.JSONResponse(http.StatusOK, "success", nil)
		return nil
	})

	req := httptest.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// ===========================================================================
// Double Write Protection
// ===========================================================================

func TestResponseWriter_DoubleWriteHeader(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder()}

	rw.WriteHeader(http.StatusOK)
	rw.WriteHeader(http.StatusInternalServerError) // should be ignored

	if !rw.headerWritten {
		t.Error("expected headerWritten to be true")
	}
}

func TestResponseWriter_WriteImplicitHeader(t *testing.T) {
	inner := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: inner}

	rw.Write([]byte("hello"))

	if !rw.headerWritten {
		t.Error("expected headerWritten to be true after Write")
	}
	if inner.Code != http.StatusOK {
		t.Errorf("expected implicit 200 status, got %d", inner.Code)
	}
}

func TestResponseWriter_HeaderWritten(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder()}

	if rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be false initially")
	}

	rw.WriteHeader(http.StatusOK)

	if !rw.HeaderWritten() {
		t.Error("expected HeaderWritten to be true after WriteHeader")
	}
}

// ===========================================================================
// PrintRoutes (smoke test — just ensure no panic)
// ===========================================================================

func TestRouter_PrintRoutes(t *testing.T) {
	r := NewRouter()
	r.GET("/a", func(c *context.Context) error { return nil })
	r.POST("/b", func(c *context.Context) error { return nil })
	r.DELETE("/c", func(c *context.Context) error { return nil })
	r.OPTIONS("/d", func(c *context.Context) error { return nil })

	// Should not panic
	r.PrintRoutes()
}

// ===========================================================================
// Route Registration
// ===========================================================================

func TestRouter_RoutesTracked(t *testing.T) {
	r := NewRouter()
	r.GET("/a", func(c *context.Context) error { return nil })
	r.POST("/b", func(c *context.Context) error { return nil })

	api := r.Group("/api")
	api.GET("/c", func(c *context.Context) error { return nil })

	if len(*r.routes) != 3 {
		t.Errorf("expected 3 registered routes, got %d", len(*r.routes))
	}

	expected := []RouteEntry{
		{Method: "GET", Path: "/a"},
		{Method: "POST", Path: "/b"},
		{Method: "GET", Path: "/api/c"},
	}

	for i, e := range expected {
		if (*r.routes)[i].Method != e.Method || (*r.routes)[i].Path != e.Path {
			t.Errorf("route[%d] expected %s %s, got %s %s",
				i, e.Method, e.Path, (*r.routes)[i].Method, (*r.routes)[i].Path)
		}
	}
}
