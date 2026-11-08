package main

import (
	"context"

	"github.com/egoist/mygo"
)

// NotificationOptions mirrors packages/desktop showNotification. `icon` (a URL
// on the TS side) is not forwarded: mygo notifications take an embedded image
// ([]byte) and never fetch remote ones.
type NotificationOptions struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// notificationClickEvent carries the clicked notification's ID to the pages;
// the web side listens on "notification.click" and matches the ID.
var notificationClickEvent = mygo.NewEvent[string]("notification.click")

// Notification exposes native desktop notifications.
//
// Show returns the notification's ID (used as the click-event correlation key
// by packages/desktop notification.ts). macOS asks for permission on the first
// Show, so there is no separate permission query — RequestPermission reports
// "granted" like a browser that shows its prompt at first use.
type Notification struct{}

func (Notification) Show(ctx context.Context, options NotificationOptions) (string, error) {
	n := mygo.NewNotification(mygo.NotificationOptions{
		Title: options.Title,
		Body:  options.Body,
	})
	n.OnClick(func() {
		_ = notificationClickEvent.Broadcast(n.ID())
	})
	if err := n.Show(); err != nil {
		return "", err
	}
	return n.ID(), nil
}

func (Notification) RequestPermission(ctx context.Context) string {
	return "granted"
}
