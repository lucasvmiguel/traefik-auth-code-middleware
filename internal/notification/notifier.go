package notification

// Notifier is the interface for sending authentication codes.
type Notifier interface {
	SendCode(code, ip string) error
}
