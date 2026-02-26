package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("Allowed Origin", func(t *testing.T) {
		middleware := Handler("https://example.com,http://localhost:3000")
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Errorf("Expected Access-Control-Allow-Origin to be http://localhost:3000, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("Disallowed Origin", func(t *testing.T) {
		middleware := Handler("https://example.com")
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://evil.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("Expected no Access-Control-Allow-Origin header, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("Wildcard Origin", func(t *testing.T) {
		middleware := Handler("*")
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://anywhere.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "http://anywhere.com" {
			t.Errorf("Expected Access-Control-Allow-Origin to be http://anywhere.com for wildcard config, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("OPTIONS Preflight", func(t *testing.T) {
		middleware := Handler("*")
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "http://anywhere.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("Expected status 204 No Content for OPTIONS, got %d", rr.Code)
		}

		if rr.Header().Get("Access-Control-Max-Age") == "" {
			t.Error("Expected Access-Control-Max-Age header to be set")
		}
	})
}

func TestNew(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("Custom Config", func(t *testing.T) {
		config := Config{
			AllowedOrigins:   []string{"https://custom.com"},
			AllowedMethods:   []string{"GET", "POST"},
			AllowedHeaders:   []string{"Content-Type"},
			MaxAge:           1 * time.Hour,
			AllowCredentials: false,
		}
		middleware := New(config)
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "https://custom.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
			t.Errorf("Expected Access-Control-Allow-Methods to be 'GET, POST', got %s", rr.Header().Get("Access-Control-Allow-Methods"))
		}

		if rr.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Errorf("Expected no Access-Control-Allow-Credentials header when disabled, got %s", rr.Header().Get("Access-Control-Allow-Credentials"))
		}
	})

	t.Run("AllowOriginFunc", func(t *testing.T) {
		config := Config{
			AllowOriginFunc: func(origin string) bool {
				return origin == "https://allowed.com"
			},
			AllowedMethods: []string{"GET"},
		}
		middleware := New(config)
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "https://allowed.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "https://allowed.com" {
			t.Errorf("Expected Access-Control-Allow-Origin to be https://allowed.com, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})
}

func TestDefault(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("Default allows all origins", func(t *testing.T) {
		middleware := Default()
		handler := middleware(nextHandler)

		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://anywhere.com")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "http://anywhere.com" {
			t.Errorf("Expected Access-Control-Allow-Origin to be http://anywhere.com, got %s", rr.Header().Get("Access-Control-Allow-Origin"))
		}

		if rr.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Error("Expected no Access-Control-Allow-Credentials when using Default()")
		}
	})
}
