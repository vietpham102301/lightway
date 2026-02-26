package benchmarks

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/labstack/echo/v4"
	lctx "github.com/vietpham102301/hermes/pkg/context"
	"github.com/vietpham102301/lightway/pkg/router"
)

// =============================================================================
// Lightway Router
// =============================================================================

func setupLightway() http.Handler {
	r := router.NewRouter()
	r.GET("/", func(c *lctx.Context) error {
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("OK"))
		return nil
	})
	r.GET("/users/{id}", func(c *lctx.Context) error {
		_ = c.Param("id")
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("OK"))
		return nil
	})
	r.POST("/users", func(c *lctx.Context) error {
		c.W.WriteHeader(http.StatusCreated)
		c.W.Write([]byte("Created"))
		return nil
	})
	r.GET("/users/{id}/posts/{postId}", func(c *lctx.Context) error {
		_ = c.Param("id")
		_ = c.Param("postId")
		c.W.WriteHeader(http.StatusOK)
		c.W.Write([]byte("OK"))
		return nil
	})
	return r
}

// =============================================================================
// net/http (stdlib)
// =============================================================================

func setupStdlib() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = r.PathValue("id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})
	mux.HandleFunc("GET /users/{id}/posts/{postId}", func(w http.ResponseWriter, r *http.Request) {
		_ = r.PathValue("id")
		_ = r.PathValue("postId")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	return mux
}

// =============================================================================
// Gin
// =============================================================================

func setupGin() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	r.GET("/users/:id", func(c *gin.Context) {
		_ = c.Param("id")
		c.String(http.StatusOK, "OK")
	})
	r.POST("/users", func(c *gin.Context) {
		c.String(http.StatusCreated, "Created")
	})
	r.GET("/users/:id/posts/:postId", func(c *gin.Context) {
		_ = c.Param("id")
		_ = c.Param("postId")
		c.String(http.StatusOK, "OK")
	})
	return r
}

// =============================================================================
// Chi
// =============================================================================

func setupChi() http.Handler {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		_ = chi.URLParam(r, "id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	})
	r.Get("/users/{id}/posts/{postId}", func(w http.ResponseWriter, r *http.Request) {
		_ = chi.URLParam(r, "id")
		_ = chi.URLParam(r, "postId")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	return r
}

// =============================================================================
// Echo
// =============================================================================

func setupEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})
	e.GET("/users/:id", func(c echo.Context) error {
		_ = c.Param("id")
		return c.String(http.StatusOK, "OK")
	})
	e.POST("/users", func(c echo.Context) error {
		return c.String(http.StatusCreated, "Created")
	})
	e.GET("/users/:id/posts/:postId", func(c echo.Context) error {
		_ = c.Param("id")
		_ = c.Param("postId")
		return c.String(http.StatusOK, "OK")
	})
	return e
}

// =============================================================================
// Benchmark: Static Route — GET /
// =============================================================================

func BenchmarkStaticRoute_Lightway(b *testing.B) {
	h := setupLightway()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkStaticRoute_Stdlib(b *testing.B) {
	h := setupStdlib()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkStaticRoute_Gin(b *testing.B) {
	h := setupGin()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkStaticRoute_Chi(b *testing.B) {
	h := setupChi()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkStaticRoute_Echo(b *testing.B) {
	e := setupEcho()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

// =============================================================================
// Benchmark: Parameterized Route — GET /users/{id}
// =============================================================================

func BenchmarkParamRoute_Lightway(b *testing.B) {
	h := setupLightway()
	req := httptest.NewRequest("GET", "/users/123", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkParamRoute_Stdlib(b *testing.B) {
	h := setupStdlib()
	req := httptest.NewRequest("GET", "/users/123", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkParamRoute_Gin(b *testing.B) {
	h := setupGin()
	req := httptest.NewRequest("GET", "/users/123", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkParamRoute_Chi(b *testing.B) {
	h := setupChi()
	req := httptest.NewRequest("GET", "/users/123", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkParamRoute_Echo(b *testing.B) {
	e := setupEcho()
	req := httptest.NewRequest("GET", "/users/123", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

// =============================================================================
// Benchmark: Multi-Param Route — GET /users/{id}/posts/{postId}
// =============================================================================

func BenchmarkMultiParamRoute_Lightway(b *testing.B) {
	h := setupLightway()
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkMultiParamRoute_Stdlib(b *testing.B) {
	h := setupStdlib()
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkMultiParamRoute_Gin(b *testing.B) {
	h := setupGin()
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkMultiParamRoute_Chi(b *testing.B) {
	h := setupChi()
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkMultiParamRoute_Echo(b *testing.B) {
	e := setupEcho()
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}
