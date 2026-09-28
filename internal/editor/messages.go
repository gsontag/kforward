package editor

import "github.com/nicksnyder/go-i18n/v2/i18n"

// The English texts of the problems shown under the fields.
var (
	msgNameRequired = &i18n.Message{ID: "EditorNameRequired", Other: "A name is required"}
	msgTarget       = &i18n.Message{
		ID: "EditorTargetInvalid", Description: "pod, svc, deploy and sts are kubectl kinds: not to translate",
		Other: "Expected KIND/NAME, such as svc/grafana: KIND is pod, svc, deploy or sts",
	}
	msgLocalPort = &i18n.Message{
		ID:    "EditorLocalPortInvalid",
		Other: "A port number, from 1 to 65535",
	}
	msgRemotePort = &i18n.Message{
		ID:    "EditorRemotePortInvalid",
		Other: "A port number, or a port name made of lowercase letters, digits and hyphens",
	}
	msgAddress = &i18n.Message{
		ID:    "EditorAddressInvalid",
		Other: "An IP address, such as 127.0.0.1, or localhost",
	}
	msgURL = &i18n.Message{
		ID:    "EditorURLInvalid",
		Other: "An address starting with http:// or https://",
	}
)
