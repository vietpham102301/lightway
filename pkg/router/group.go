package router

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/vietpham102301/lightway/pkg/context"
	aerror "github.com/vietpham102301/lightway/pkg/errors"
)

// color returns the ANSI escape code if colors are enabled, or empty string otherwise.
// Respects the NO_COLOR (https://no-color.org/) convention and TERM=dumb.
func color(code string) string {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return ""
	}
	return code
}

const (
	ansiGreen  = "\033[32m"
	ansiWhite  = "\033[37m"
	ansiBlue   = "\033[34m"
	ansiYellow = "\033[33m"
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiPurple = "\033[35m"
)

type HandlerFunc func(c *context.Context) error
type Middleware func(http.Handler) http.Handler

type RouteEntry struct {
	Method string
	Path   string
}

// responseWriter wraps http.ResponseWriter to track if headers were written
type responseWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.headerWritten {
		rw.headerWritten = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) HeaderWritten() bool {
	return rw.headerWritten
}

type Router struct {
	mux         *http.ServeMux
	prefix      string
	middlewares []Middleware
	routes      *[]RouteEntry
}

func NewRouter() *Router {
	return &Router{
		mux:         http.NewServeMux(),
		prefix:      "",
		middlewares: []Middleware{},
		routes:      &[]RouteEntry{},
	}
}

func (r *Router) Group(path string) *Router {
	return &Router{
		mux:         r.mux,
		prefix:      r.prefix + path,
		middlewares: append([]Middleware(nil), r.middlewares...),
		routes:      r.routes,
	}
}

func (r *Router) Use(mw ...Middleware) {
	r.middlewares = append(r.middlewares, mw...)
}

func (r *Router) Handle(method, path string, handler HandlerFunc) {
	standardHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w}
		ctx := &context.Context{
			W: rw,
			R: r,
		}
		err := handler(ctx)
		if err != nil {
			if !rw.headerWritten {
				var appErr *aerror.AppError
				if errors.As(err, &appErr) {
					ctx.JSONResponse(appErr.Code, nil, appErr)
					return
				}

				ctx.JSONResponse(http.StatusInternalServerError, nil, errors.New("internal server error"))
			}
		}
	})

	finalHandler := http.Handler(standardHandler)
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		finalHandler = r.middlewares[i](finalHandler)
	}

	fullPattern := method + " " + r.prefix + path
	displayPath := r.prefix + path
	if displayPath == "" {
		displayPath = "/"
	}
	*r.routes = append(*r.routes, RouteEntry{
		Method: method,
		Path:   displayPath,
	})

	r.mux.Handle(fullPattern, finalHandler)
}

func (r *Router) PrintRoutes() {
	for _, route := range *r.routes {
		methodColor := ansiGreen
		if route.Method == "POST" || route.Method == "PUT" {
			methodColor = ansiYellow
		} else if route.Method == "DELETE" {
			methodColor = ansiRed
		} else if route.Method == "OPTIONS" {
			methodColor = ansiPurple
		}

		fmt.Printf("%s[Router] %s%-7s%s %s%s%s\n",
			color(ansiWhite),
			color(methodColor), route.Method,
			color(ansiReset),
			color(ansiBlue), route.Path,
			color(ansiReset),
		)
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// WithMiddleware wraps the router with additional middlewares and returns an http.Handler.
func (r *Router) WithMiddleware(middlewares ...Middleware) http.Handler {
	handler := http.Handler(r)
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func (r *Router) GET(path string, handler HandlerFunc) {
	r.Handle("GET", path, handler)
}

func (r *Router) POST(path string, handler HandlerFunc) {
	r.Handle("POST", path, handler)
}

func (r *Router) PUT(path string, handler HandlerFunc) {
	r.Handle("PUT", path, handler)
}

func (r *Router) DELETE(path string, handler HandlerFunc) {
	r.Handle("DELETE", path, handler)
}

func (r *Router) OPTIONS(path string, handler HandlerFunc) {
	r.Handle("OPTIONS", path, handler)
}
