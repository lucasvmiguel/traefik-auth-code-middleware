package logger

import "log"

type Notifier struct{}

func New() *Notifier {
	return &Notifier{}
}

func (l *Notifier) SendCode(code, ip string) error {
	log.Printf("CODE GENERATED for %s: %s", ip, code)
	return nil
}
