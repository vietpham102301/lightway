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

HTTP client wrapper with connection pooling and automatic error handling.

```go
import "github.com/vietpham102301/lightway/pkg/httpclient"

client := httpclient.NewClient()

// Send request and receive response bytes
body, err := client.RequestBytes(
    ctx,
    http.MethodPost,
    "https://api.example.com/data",
    payload,                            // body (auto-marshaled to JSON)
    map[string]string{                  // headers
        "Authorization": "Bearer token",
    },
)

// Or use Do() for full control
req, _ := http.NewRequest("GET", url, nil)
resp, err := client.Do(req)
defer resp.Body.Close()
```

**Default settings:** Max Idle Conns: 100 · Per Host: 10 · Idle Timeout: 90s · Request Timeout: 60s

---

### Notifier (Telegram)

Send notifications via the Telegram Bot API.

```go
import "github.com/vietpham102301/lightway/pkg/notifier"

telegramNotifier := notifier.NewTelegramNotifier(
    httpClient,   // *httpclient.Client
    botToken,     // Telegram Bot token
    chatID,       // Target chat ID
)

err := telegramNotifier.SendNotifyTelegram("✅ Deployment completed!")
```

---

## 📊 Benchmarks

Router performance compared against popular Go frameworks.

**Environment:** Apple M2 Pro · Go 1.25.6 · macOS · `go test -bench=. -benchmem -count=3`

### Static Route — `GET /`

| Framework | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| **net/http (stdlib)** | 159 | 274 | 6 |
| **Lightway** | 189 | 322 | 8 |
| Chi | 265 | 642 | 8 |
| Gin | 410 | 1040 | 9 |
| Echo | 483 | 1016 | 10 |

### Parameterized Route — `GET /users/{id}`

| Framework | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| **net/http (stdlib)** | 233 | 290 | 7 |
| **Lightway** | 283 | 338 | 9 |
| Chi | 403 | 978 | 10 |
| Gin | 420 | 1040 | 9 |
| Echo | 495 | 1016 | 10 |

### Multi-Param Route — `GET /users/{id}/posts/{postId}`

| Framework | ns/op | B/op | allocs/op |
|-----------|------:|-----:|----------:|
| **net/http (stdlib)** | 351 | 322 | 8 |
| **Lightway** | 388 | 370 | 10 |
| Chi | 443 | 978 | 10 |
| Gin | 462 | 1040 | 9 |
| Echo | 512 | 1016 | 10 |

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
