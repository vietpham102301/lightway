# ⚡ Lightway

**Lightway** is a collection of reusable Go utility packages for building backend web applications quickly and consistently.

## 📦 Installation

```bash
go get github.com/vietpham102301/lightway
```

> **Requires:** Go 1.25.6+

---

## 📚 Packages

| Package | Description |
|---------|-------------|
| `cache` | Redis client initialization with connection pooling |
| `context` | HTTP context wrapper, JSON response & request binding |
| `cors` | Flexible CORS middleware for HTTP servers |
| `errors` | Structured application errors with HTTP status codes |
| `httpclient` | HTTP client wrapper with connection pooling |
| `jwt` | JWT token generation using RS256 algorithm |
| `logger` | Structured logging based on `log/slog` |
| `notifier` | Send notifications via Telegram Bot API |
| `pool` | Generic, dynamically-scaling worker pool |
| `router` | HTTP router with route groups & middleware chain |
| `sql` | PostgreSQL connection pool initialization (pgxpool) |

---

## 🚀 Usage

### Router

Built on top of `http.ServeMux` with support for route groups, middleware chaining, and automatic error response handling.

```go
import (
    "github.com/vietpham102301/lightway/pkg/router"
    lctx "github.com/vietpham102301/lightway/pkg/context"
)

r := router.NewRouter()

// Register global middleware
r.Use(corsMiddleware, loggingMiddleware)

// Route groups
api := r.Group("/api/v1")
api.GET("/users", func(c *lctx.Context) error {
    return c.JSONResponse(200, map[string]string{"message": "hello"}, nil)
})
api.POST("/users", createUserHandler)

// Print registered routes to console (colorized)
r.PrintRoutes()

// Start the server
http.ListenAndServe(":8080", r)
```

---

### Context

Wraps `http.ResponseWriter` and `*http.Request` to simplify request/response handling.

```go
import lctx "github.com/vietpham102301/lightway/pkg/context"

func GetUserHandler(c *lctx.Context) error {
    // Path parameter
    id, err := c.ParamInt("id")
    if err != nil {
        return errors.InvalidRequest(err)
    }

    // Query parameters
    page := c.QueryInt("page", 1)
    search := c.Query("search")

    // Bind JSON request body
    var body CreateUserRequest
    if err := c.BindJSON(&body); err != nil {
        return errors.InvalidRequest(err)
    }

    // Get user ID from context (set by auth middleware)
    userID, err := c.GetUserID()

    // Send JSON response
    return c.JSONResponse(http.StatusOK, user, nil)
}
```

**Standard response format:**

```json
{
  "code": 200,
  "data": { ... },
  "error": ""
}
```

---

### CORS

Configurable CORS middleware with sensible defaults.

```go
import "github.com/vietpham102301/lightway/pkg/cors"

// Option 1: Allow all origins (credentials disabled)
r.Use(cors.Default())

// Option 2: From a comma-separated string
r.Use(cors.Handler("https://example.com,https://app.example.com"))

// Option 3: Full configuration
r.Use(cors.New(cors.Config{
    AllowedOrigins:   []string{"https://example.com"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders:   []string{"Origin", "Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           24 * time.Hour,
}))

// Option 4: Custom validation function
r.Use(cors.New(cors.Config{
    AllowOriginFunc: func(origin string) bool {
        return strings.HasSuffix(origin, ".example.com")
    },
}))
```

---

### Errors

Provides a structured `AppError` type that maps to HTTP status codes. When a handler returns an `AppError`, the router automatically serializes it into a JSON response.

```go
import "github.com/vietpham102301/lightway/pkg/errors"

// Built-in factory functions
errors.InvalidRequest(err)            // 400 Bad Request
errors.NotFound("user not found")     // 404 Not Found
errors.Unauthorized("invalid token")  // 401 Unauthorized
errors.InternalServerError()          // 500 Internal Server Error

// Custom error
errors.NewAppError(http.StatusForbidden, "Forbidden", originalErr)
```

---

### Logger

Structured logging wrapper built on Go's standard `log/slog`.

```go
import "github.com/vietpham102301/lightway/pkg/logger"

// Initialize once at app startup
logger.Init(logger.Config{
    Level:  "info",   // debug | info | warn | error
    Format: "json",   // json | text
})

// Usage
logger.Info("server started", "port", 8080)
logger.Error("failed to connect", logger.Err(err))
logger.Debug("processing request", "method", "GET", "path", "/api/users")
logger.Warn("slow query", "duration", "2.5s")

// Logger with fixed attributes
log := logger.With("component", "auth")
log.Info("user logged in", "user_id", 123)
```

---

### Cache (Redis)

Creates a Redis client with pre-configured connection pooling.

```go
import "github.com/vietpham102301/lightway/pkg/cache"

redisClient, err := cache.NewRedisClient(cfg)
if err != nil {
    log.Fatal(err)
}

// Use the client as usual
redisClient.Set(ctx, "key", "value", time.Hour)
val, err := redisClient.Get(ctx, "key").Result()
```

**Default settings:** Pool Size: 10 · Min Idle Conns: 5 · Max Retries: 3 · Connect Timeout: 5s

---

### SQL (PostgreSQL)

Creates a PostgreSQL connection pool using `pgxpool`.

```go
import "github.com/vietpham102301/lightway/pkg/sql"

pool, err := sql.NewPostgresDB(connString, postgresConf)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

rows, err := pool.Query(ctx, "SELECT * FROM users WHERE id = $1", userID)
```

**Configuration:**
- `MaxConn` — Maximum number of connections
- `ConnMaxLifetime` — Max connection lifetime (minutes)
- Health check interval: 1 min · Connect timeout: 5s

---

### JWT

Generates JWT tokens using the RS256 signing algorithm with an RSA private key.

```go
import "github.com/vietpham102301/lightway/pkg/jwt"

token, err := jwt.GenerateToken(
    privateKey,   // *rsa.PrivateKey
    userID,       // int
    username,     // string
    role,         // string
    24,           // expires in 24 hours
)
```

**Token claims:** `user_id`, `username`, `role`, `exp`, `iat`

---

### HTTP Client

HTTP client wrapper with connection pooling, automatic error handling, and **retry with exponential backoff**.

```go
import "github.com/vietpham102301/lightway/pkg/httpclient"

// Basic client (no retry)
client := httpclient.NewClient()

// Client with custom config
client := httpclient.NewClientWithConfig(httpclient.Config{
    MaxIdleConns:        200,
    MaxIdleConnsPerHost: 20,
    Timeout:             30 * time.Second,
})

// Enable retry with exponential backoff
client := httpclient.NewClient().WithRetry(httpclient.RetryConfig{
    MaxRetries: 3,                    // up to 3 retries (4 total attempts)
    BaseDelay:  500 * time.Millisecond, // 500ms → 1s → 2s → ...
    MaxDelay:   10 * time.Second,     // cap at 10s
    RetryOn:    []int{429, 502, 503, 504}, // default retryable status codes
})

// Send request
body, err := client.RequestBytes(ctx, http.MethodPost, url, payload, headers)

// Custom retry decision
client := httpclient.NewClient().WithRetry(httpclient.RetryConfig{
    MaxRetries: 5,
    BaseDelay:  100 * time.Millisecond,
    ShouldRetry: func(resp *http.Response, err error) bool {
        return err != nil || resp.StatusCode == http.StatusConflict
    },
})
```

**Default retry behavior:** retries on 429 (Rate Limit), 502, 503, 504 with exponential backoff. Context cancellation is respected between retries.

---

### Worker Pool

A generic, dynamically-scaling worker pool. Add a new job type by implementing a single `Job[T]` interface — the pool handles worker lifecycle, panic recovery, graceful shutdown, and auto-scaling.

```go
import "github.com/vietpham102301/lightway/pkg/pool"
```

**Step 1 — implement `Job[T]`:**

```go
type SendEmailJob struct {
    To      string
    Subject string
    Body    string
}

func (j SendEmailJob) Execute(ctx context.Context) (string, error) {
    if ctx.Err() != nil {
        return "", ctx.Err()
    }
    emailID, err := emailService.Send(j.To, j.Subject, j.Body)
    return emailID, err
}
```

**Step 2 — create and start the pool:**

```go
p := pool.New[string](pool.Config{
    MinWorkers:  2,   // always-on goroutines
    MaxWorkers:  20,  // scale up to 20 under load
    QueueSize:   500, // buffered job queue
})
p.Start()
defer p.Stop() // graceful shutdown — waits for running jobs to finish
```

**Step 3 — submit jobs and receive results:**

```go
resultCh, err := p.Submit(SendEmailJob{To: "user@example.com", Subject: "Hi", Body: "..."})
switch {
case errors.Is(err, pool.ErrQueueFull):
    // queue at capacity — retry, drop, or log
case err != nil:
    // pool is stopped
default:
    res := <-resultCh
    if res.Err != nil {
        log.Printf("failed: %v", res.Err)
    } else {
        log.Printf("email sent, ID: %s", res.Value)
    }
}
```

**Auto-scaling behavior:**
- Scales **up** when queue depth grows — spawns up to `MaxWorkers` goroutines
- Scales **down** automatically — workers above `MinWorkers` exit after `IdleTimeout` of inactivity
- Panicking jobs are **recovered** without killing the worker; the pool keeps running

**Observability:**

```go
snap := p.Stats()
// snap.ActiveWorkers — live goroutines
// snap.QueueDepth    — pending jobs
// snap.Processed     — total completed (success + error)
// snap.Failed        — jobs that returned non-nil error
// snap.Panics        — jobs recovered from panic
```

**Config defaults** (all zero values are safe):

| Field | Default |
|-------|---------|
| `MinWorkers` | `2` |
| `MaxWorkers` | `runtime.NumCPU() × 2` |
| `QueueSize` | `MaxWorkers × 10` |
| `IdleTimeout` | `30s` |
| `ScaleInterval` | `100ms` |

---

### Notifier

Send notifications via pluggable notifier implementations.

```go
import "github.com/vietpham102301/lightway/pkg/notifier"

// Notifier interface — implement for any channel
var n notifier.Notifier

// Telegram implementation
n = notifier.NewTelegramNotifier(httpClient, botToken, chatID)
err := n.Send("✅ Deployment completed!")
```

---

## 📊 Benchmarks

Router performance compared against popular Go frameworks.

**Environment:** Apple M2 Pro · Go 1.25.6 · macOS · `go test -bench=. -benchmem -count=3`

### Static Route — `GET /`

| Framework | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| **net/http (stdlib)** | 159 | 274 | 6 |
| **Lightway** | 191 | 322 | 8 |
| Chi | 240 | 642 | 8 |
| Gin | 389 | 1040 | 9 |
| Echo | 412 | 1016 | 10 |

### Parameterized Route — `GET /users/{id}`

| Framework | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| **net/http (stdlib)** | 226 | 290 | 7 |
| **Lightway** | 260 | 338 | 9 |
| Chi | 362 | 978 | 10 |
| Gin | 397 | 1040 | 9 |
| Echo | 432 | 1016 | 10 |

### Multi-Param Route — `GET /users/{id}/posts/{postId}`

| Framework | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| **net/http (stdlib)** | 314 | 322 | 8 |
| **Lightway** | 352 | 370 | 10 |
| Chi | 398 | 978 | 10 |
| Gin | 407 | 1040 | 9 |
| Echo | 446 | 1016 | 10 |

> **Key takeaways:**
> - Lightway performs within **~15-20%** of Go's standard library — the thinnest wrapper overhead among all tested frameworks.
> - **~2x faster** than Gin and Echo on static routes with **3x less memory** allocation.
> - Consistently uses the **fewest bytes per operation** among third-party routers.

Run the benchmarks yourself:

```bash
go test -bench=. -benchmem -count=3 ./benchmarks/
```

---

## 🏗️ Full Example

A minimal web application using multiple Lightway packages together:

```go
package main

import (
    "net/http"

    "github.com/vietpham102301/lightway/pkg/cors"
    lctx "github.com/vietpham102301/lightway/pkg/context"
    "github.com/vietpham102301/lightway/pkg/errors"
    "github.com/vietpham102301/lightway/pkg/logger"
    "github.com/vietpham102301/lightway/pkg/router"
)

func main() {
    // 1. Initialize logger
    logger.Init(logger.Config{Level: "info", Format: "json"})

    // 2. Create router
    r := router.NewRouter()

    // 3. Register middleware
    r.Use(cors.Handler("http://localhost:3000"))

    // 4. Register routes
    api := r.Group("/api/v1")

    api.GET("/health", func(c *lctx.Context) error {
        return c.JSONResponse(200, map[string]string{"status": "ok"}, nil)
    })

    api.GET("/users/{id}", func(c *lctx.Context) error {
        id, err := c.ParamInt("id")
        if err != nil {
            return errors.InvalidRequest(err)
        }
        return c.JSONResponse(200, map[string]int{"user_id": id}, nil)
    })

    // 5. Print routes & start server
    r.PrintRoutes()
    logger.Info("server starting", "port", 8080)
    http.ListenAndServe(":8080", r)
}
```

---

## 📄 License

MIT
