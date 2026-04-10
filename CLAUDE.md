# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/router/...

# Run tests with verbose output
go test -v ./pkg/pool/...

# Run benchmarks (in the separate benchmarks module)
cd benchmarks && go test -bench=. -benchmem -count=3 ./...

# Run a single test by name
go test -run TestFunctionName ./pkg/router/...
```

## Architecture

Lightway is a collection of independent Go utility packages under `pkg/`. Each package is self-contained and can be imported separately via `github.com/vietpham102301/lightway/pkg/<name>`.

The `benchmarks/` directory is a **separate Go module** (`benchmarks/go.mod`) that imports the main module to benchmark Lightway's router against other frameworks (Chi, Gin, Echo).

### Key package relationships

**`router` → `context` + `errors`**: The router wraps `http.ServeMux`. Each route handler receives a `*context.Context` (not the stdlib `context.Context`). When a handler returns an error, the router checks if it's an `*errors.AppError` and auto-serializes it as JSON. Non-`AppError` errors become 500 responses.

**`context`**: Wraps `http.ResponseWriter` + `*http.Request`. All JSON responses use the `AppResponse` struct: `{"code": N, "data": ..., "error": "..."}`. The router uses a `responseWriter` wrapper that tracks whether headers were written to prevent double-writes.

**`pool`**: Generic worker pool (`Pool[T]`) parameterized on result type. Jobs implement `Job[T]` with a single `Execute(ctx) (T, error)` method. The pool auto-scales between `MinWorkers` and `MaxWorkers` based on queue depth, with a separate scaler goroutine running every `ScaleInterval`. Panics in jobs are recovered without killing the worker.

**`errors`**: Defines `AppError` with `Code int` and `Message string`. Factory functions: `InvalidRequest`, `NotFound`, `Unauthorized`, `InternalServerError`, `NewAppError`.

**`cors`**: Middleware with three entry points — `Default()` (allow all, no credentials), `Handler(origins string)` (comma-separated origins), `New(Config)` (full config with optional `AllowOriginFunc`).

**`logger`**: Thin wrapper around `log/slog`. Call `logger.Init(Config)` once at startup. Global functions: `Info`, `Error`, `Debug`, `Warn`, `With`. `logger.Err(err)` is a helper attr for structured error logging.

**`jwt`**: RS256 token generation only (no verification). Claims: `user_id`, `username`, `role`, `exp`, `iat`.

**`httpclient`**: Wraps `http.Client` with connection pooling. `WithRetry(RetryConfig)` adds exponential backoff. `RequestBytes` is the primary method — returns raw `[]byte` body.

**`cache`** / **`sql`**: Thin initialization wrappers for `go-redis` and `pgxpool` respectively with pre-configured pool settings.

**`notifier`**: `Notifier` interface with a single `Send(message string) error` method. `TelegramNotifier` is the only implementation.
