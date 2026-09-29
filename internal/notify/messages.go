package notify

import "github.com/nicksnyder/go-i18n/v2/i18n"

// The English texts of the notifications.
var (
	msgFailed  = &i18n.Message{ID: "NotifyForwardFailed", Other: "{{.Name}}: forward stopped"}
	msgProblem = &i18n.Message{ID: "NotifyConfigProblem", Other: "Configuration problem"}
)
