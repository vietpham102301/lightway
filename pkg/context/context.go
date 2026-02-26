package context

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/vietpham102301/lightway/pkg/logger"
)

type contextKey string

// UserIDKey are the single source of truth for request context keys.
const (
	UserIDKey contextKey = "user_id"
)

type Context struct {
	W http.ResponseWriter
	R *http.Request
}

type AppResponse struct {
	Code  int    `json:"code"`
	Data  any    `json:"data"`
	Error string `json:"error"`
}

func (c *Context) JSONResponse(status int, data any, err error) {
	if rw, ok := c.W.(interface{ HeaderWritten() bool }); ok && rw.HeaderWritten() {
		return
	}

	c.W.Header().Set("Content-Type", "application/json")
	c.W.WriteHeader(status)

	enc := json.NewEncoder(c.W)
	enc.SetIndent("", "  ")

	formatedResponse := AppResponse{
		Code: status,
		Data: data,
	}

	if err != nil {
		formatedResponse.Error = err.Error()
	}

	if err := enc.Encode(formatedResponse); err != nil {
		logger.Error("encoding json failed", logger.Err(err))
	}
}

// WriteErrorResponse writes a JSON error response with the same format as AppResponse (code, data, error).
func WriteErrorResponse(w http.ResponseWriter, status int, message string, _ error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(AppResponse{
		Code:  status,
		Data:  nil,
		Error: message,
	})
}

func (c *Context) BindJSON(v any) error {
	return json.NewDecoder(c.R.Body).Decode(v)
}

func (c *Context) Param(key string) string {
	return c.R.PathValue(key)
}

func (c *Context) ParamInt(key string) (int, error) {
	val := c.R.PathValue(key)
	if val == "" {
		return -1, errors.New("no value received")
	}

	res, err := strconv.Atoi(val)
	if err != nil {
		return -1, err
	}
	return res, nil
}

func (c *Context) Query(key string) string {
	return c.R.URL.Query().Get(key)
}

func (c *Context) QueryInt(key string, defaultValue int) int {
	val := c.R.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	res, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return res
}

func (c *Context) Status(code int) {
	c.W.WriteHeader(code)
}

func (c *Context) GetUserID() (int, error) {
	val := c.Context().Value(UserIDKey)
	if val == nil {
		return -1, errors.New("user id not found in context")
	}

	userID, ok := val.(int)
	if !ok {
		return -1, errors.New("user id is not a int")
	}

	return userID, nil
}

func (c *Context) Context() context.Context {
	return c.R.Context()
}
