package window

import "github.com/nicksnyder/go-i18n/v2/i18n"

// The English texts of the window; see internal/tray/messages.go.
var (
	msgTitle     = &i18n.Message{ID: "WindowTitle", Other: "Kube Forwarder"}
	msgEmpty     = &i18n.Message{ID: "WindowEmpty", Other: "No forward configured"}
	msgUngrouped = &i18n.Message{
		ID:          "WindowUngrouped",
		Description: "Header of the forwards without group",
		Other:       "Other",
	}
	msgStopped    = &i18n.Message{ID: "WindowStateStopped", Other: "Stopped"}
	msgConnecting = &i18n.Message{ID: "WindowStateConnecting", Other: "Connecting…"}
	msgActive     = &i18n.Message{
		ID:    "WindowStateActive",
		Other: "Active on {{.Address}}:{{.Port}}",
	}
	msgFailed   = &i18n.Message{ID: "WindowStateFailed", Other: "Failed"}
	msgRetrying = &i18n.Message{
		ID: "WindowStateRetrying", Description: "Reconnecting after failures",
		One: "Reconnecting, {{.Count}} failure", Other: "Reconnecting, {{.Count}} failures",
	}
	msgTarget = &i18n.Message{
		ID:    "WindowTarget",
		Other: "{{.Context}} · {{.Namespace}}/{{.Target}}",
	}
)
