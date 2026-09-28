package tray

import "github.com/nicksnyder/go-i18n/v2/i18n"

// The English texts of the menu. goi18n extract collects them into the
// translation files: an ID must never change once translated.
var (
	msgProblem = &i18n.Message{ID: "TrayProblem", Other: "⚠️ {{.Problem}}"}
	msgEmpty   = &i18n.Message{ID: "TrayEmpty", Other: "No forward configured"}
	msgOther   = &i18n.Message{
		ID:          "TrayUngrouped",
		Description: "Header of the forwards without group",
		Other:       "Other",
	}
	msgForward    = &i18n.Message{ID: "TrayForward", Other: "{{.Name}}  :{{.Port}}"}
	msgConnecting = &i18n.Message{
		ID:    "TrayForwardConnecting",
		Other: "{{.Name}}  :{{.Port}} — connecting…",
	}
	msgFailed = &i18n.Message{ID: "TrayForwardFailed", Other: "{{.Name}}  :{{.Port}} — failed"}
	msgTarget = &i18n.Message{
		ID:    "TrayForwardTarget",
		Other: "{{.Target}} ➔ localhost:{{.Port}}",
	}
	msgOpen    = &i18n.Message{ID: "TrayOpenInBrowser", Other: "Open in browser"}
	msgStopAll = &i18n.Message{ID: "TrayStopAll", Other: "Stop all"}
	msgEdit    = &i18n.Message{ID: "TrayEditConfig", Other: "Edit configuration…"}
	msgQuit    = &i18n.Message{ID: "TrayQuit", Other: "Quit"}
	msgActive  = &i18n.Message{
		ID: "TrayActiveCount", Description: "Tooltip of the icon",
		One: "{{.Count}} active forward", Other: "{{.Count}} active forwards",
	}
	msgShowWindow = &i18n.Message{ID: "TrayShowWindow", Other: "Open window…"}
)
