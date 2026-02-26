package notifier

// Notifier is the interface for sending notifications.
type Notifier interface {
	Send(message string) error
}
