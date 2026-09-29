package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	service = "org.freedesktop.Notifications"
	path    = "/org/freedesktop/Notifications"
	timeout = time.Second
)

// Sender shows notifications through the freedesktop notification service,
// which every desktop provides. It is used from the UI thread only.
type Sender struct {
	appName      string
	desktopEntry string
	icon         string
	conn         *dbus.Conn
	// ids of the notifications shown, by key, to replace them
	ids map[string]uint32
}

// NewSender connects to the session bus; without one, notifications are
// only logged. desktopEntry names the .desktop file of the application.
func NewSender(appName, desktopEntry, icon string) *Sender {
	s := &Sender{appName: appName, desktopEntry: desktopEntry, icon: icon, ids: map[string]uint32{}}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		slog.Warn("notifications disabled", "err", err)
		return s
	}
	s.conn = conn
	return s
}

// Send shows n, replacing the previous notification with the same key.
func (s *Sender) Send(n Notification) {
	if s.conn == nil {
		slog.Info("notification", "summary", n.Summary, "body", n.Body)
		return
	}
	// A notification service that hangs must not freeze the application
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var id uint32
	hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(s.desktopEntry)}
	err := s.conn.Object(service, path).
		CallWithContext(ctx, service+".Notify", 0, s.appName, s.ids[n.Key],
			s.icon, n.Summary, n.Body, []string{}, hints, int32(-1)).
		Store(&id)
	if err != nil {
		slog.Warn("notification", "summary", n.Summary, "err", err)
		return
	}
	s.ids[n.Key] = id
}

// Close disconnects from the session bus.
func (s *Sender) Close() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
}
