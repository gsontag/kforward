package gui

import "github.com/nicksnyder/go-i18n/v2/i18n"

// The English texts of the widgets; the IDs keep the Window prefix of the
// rows they come with, see internal/window/messages.go.
var (
	msgTitle = &i18n.Message{ID: "WindowTitle", Other: "Kube Forwarder"}
	msgEmpty = &i18n.Message{ID: "WindowEmpty", Other: "No forward configured"}
)
