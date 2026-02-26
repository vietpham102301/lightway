package cors

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vietpham102301/hermes/pkg/logger"
)

const (
	// DefaultMaxAge is the default preflight cache duration (24 hours)
	DefaultMaxAge = 24 * time.Hour
)

// Config holds CORS configuration options
type Config struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present in the list, all origins will be allowed.
	// Can be a comma-separated string or []string slice.
	AllowedOrigins []string

	// AllowOriginFunc is a custom function to validate the origin.
	// If set, AllowedOrigins is ignored.
	AllowOriginFunc func(origin string) bool

	// AllowedMethods is a list of methods the client is allowed to use.
	// Default: GET, POST, PUT, DELETE, OPTIONS
	AllowedMethods []string

	// AllowedHeaders is a list of non-simple headers the client is allowed to use.
	AllowedHeaders []string

	// ExposedHeaders indicates which headers are safe to expose to the API.
	ExposedHeaders []string

	// AllowCredentials indicates whether the request can include user credentials.
	AllowCredentials bool

	// MaxAge indicates how long the results of a preflight request can be cached.
	MaxAge time.Duration
}

// DefaultConfig returns a default CORS configuration
func DefaultConfig() Config {
	return Config{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Authorization", "Accept"},
		AllowCredentials: true,
		MaxAge:           DefaultMaxAge,
	}
}

// New creates a new CORS middleware with the provided configuration.
func New(config Config) func(http.Handler) http.Handler {
	allowedMap := make(map[string]bool)
	allowAll := false

	if config.AllowOriginFunc == nil {
		for _, origin := range config.AllowedOrigins {
			trimmed := strings.TrimSpace(origin)
			if trimmed == "*" {
				allowAll = true
			}
			allowedMap[trimmed] = true
		}
	}

	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{"Origin", "Content-Type", "Authorization", "Accept"}
	}
	if config.MaxAge == 0 {
		config.MaxAge = DefaultMaxAge
	}

	methodsStr := strings.Join(config.AllowedMethods, ", ")
	headersStr := strings.Join(config.AllowedHeaders, ", ")
	exposedHeadersStr := strings.Join(config.ExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Set("Vary", "Origin")

			var isAllowed bool
			if config.AllowOriginFunc != nil {
				isAllowed = config.AllowOriginFunc(origin)
			} else {
				isAllowed = (allowAll && origin != "") || allowedMap[origin]
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				if isAllowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", methodsStr)
					w.Header().Set("Access-Control-Allow-Headers", headersStr)
					if config.AllowCredentials {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
					if config.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", formatMaxAge(config.MaxAge))
					}
					if exposedHeadersStr != "" {
						w.Header().Set("Access-Control-Expose-Headers", exposedHeadersStr)
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}

				logger.Warn("CORS forbidden", "origin", origin)
				w.WriteHeader(http.StatusForbidden)
				return
			}

			// Handle actual requests
			if isAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", methodsStr)
				w.Header().Set("Access-Control-Allow-Headers", headersStr)
				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if exposedHeadersStr != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeadersStr)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Default returns a CORS middleware with default configuration allowing all origins.
// Note: This disables credentials. For credentialed requests, use New() with specific origins.
func Default() func(http.Handler) http.Handler {
	config := DefaultConfig()
	config.AllowedOrigins = []string{"*"}
	config.AllowCredentials = false
	return New(config)
}

// Handler creates a CORS middleware from a comma-separated string of allowed origins.
func Handler(allowedOrigins string) func(http.Handler) http.Handler {
	origins := strings.Split(allowedOrigins, ",")
	config := DefaultConfig()
	config.AllowedOrigins = origins
	return New(config)
}

// formatMaxAge converts time.Duration to seconds string
func formatMaxAge(d time.Duration) string {
	return strconv.Itoa(int(d.Seconds()))
}
