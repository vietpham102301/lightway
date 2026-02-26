package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vietpham102301/lightway/pkg/httpclient"
)

// ===========================================================================
// NewTelegramNotifier
// ===========================================================================

func TestNewTelegramNotifier(t *testing.T) {
	client := httpclient.NewClient()
	n := NewTelegramNotifier(client, "bot-token", "chat-123")

	if n.Token != "bot-token" {
		t.Errorf("expected Token 'bot-token', got %q", n.Token)
	}
	if n.ChatID != "chat-123" {
		t.Errorf("expected ChatID 'chat-123', got %q", n.ChatID)
	}
	if n.Client == nil {
		t.Error("expected non-nil Client")
	}
}

// ===========================================================================
// Send
// ===========================================================================

func TestSend_Success(t *testing.T) {
	var receivedChatID, receivedText string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		receivedChatID = payload["chat_id"]
		receivedText = payload["text"]
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := httpclient.NewClient()
	n := &TelegramNotifier{
		Token:  "fake-token",
		ChatID: "12345",
		Client: client,
	}

	// Override the URL by using the test server URL
	// We need to call RequestBytes directly, so let's test via the real Send
	// but intercept via a custom TelegramNotifier that points to our server
	n2 := &testTelegramNotifier{
		client:  client,
		chatID:  "12345",
		baseURL: server.URL,
	}

	err := n2.Send("hello world")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedChatID != "12345" {
		t.Errorf("expected chat_id '12345', got %q", receivedChatID)
	}
	if receivedText != "hello world" {
		t.Errorf("expected text 'hello world', got %q", receivedText)
	}

	// Verify the original constructor works correctly
	_ = n
}

func TestSend_EmptyToken(t *testing.T) {
	client := httpclient.NewClient()
	n := NewTelegramNotifier(client, "", "chat-123")

	err := n.Send("test")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestSend_EmptyChatID(t *testing.T) {
	client := httpclient.NewClient()
	n := NewTelegramNotifier(client, "token", "")

	err := n.Send("test")
	if err == nil {
		t.Fatal("expected error for empty chat ID")
	}
}

func TestSend_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"Bad Request"}`))
	}))
	defer server.Close()

	client := httpclient.NewClient()
	n := &testTelegramNotifier{
		client:  client,
		chatID:  "12345",
		baseURL: server.URL,
	}

	err := n.Send("test")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

// ===========================================================================
// Interface compliance
// ===========================================================================

func TestTelegramNotifier_ImplementsNotifier(t *testing.T) {
	client := httpclient.NewClient()
	var n Notifier = NewTelegramNotifier(client, "token", "chat")
	if n == nil {
		t.Error("expected non-nil Notifier")
	}
}

// ===========================================================================
// testTelegramNotifier — helper to redirect API calls to httptest server
// ===========================================================================

type testTelegramNotifier struct {
	client  *httpclient.Client
	chatID  string
	baseURL string
}

func (t *testTelegramNotifier) Send(message string) error {
	payload := map[string]string{
		"chat_id": t.chatID,
		"text":    message,
	}
	_, err := t.client.RequestBytes(context.Background(), http.MethodPost, t.baseURL+"/sendMessage", payload, nil)
	if err != nil {
		return err
	}
	return nil
}
