package notification

import "context"

type NotificationDispatcher interface {
	SendPushNotification(ctx context.Context, token string, title, body string) error
	SendSMS(ctx context.Context, to string, message string) error
}

type dispatcher struct {
	// FCM or Twilio API Clients
}

func NewNotificationDispatcher() NotificationDispatcher {
	return &dispatcher{}
}

func (d *dispatcher) SendPushNotification(ctx context.Context, token string, title, body string) error {
	// TODO: Send FCM Push
	return nil
}

func (d *dispatcher) SendSMS(ctx context.Context, to string, message string) error {
	// TODO: Send Twilio SMS
	return nil
}
