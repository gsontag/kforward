package gui

import "github.com/nicksnyder/go-i18n/v2/i18n"

// The English texts of the widgets; the IDs keep the Window prefix of the
// rows they come with, see internal/window/messages.go.
var (
	msgTitle = &i18n.Message{ID: "WindowTitle", Other: "Kube Forwarder"}
	msgEmpty = &i18n.Message{ID: "WindowEmpty", Other: "No forward configured"}
)

// The English texts of the edit dialog.
var (
	msgAdd            = &i18n.Message{ID: "WindowAdd", Other: "Add a forward"}
	msgEdit           = &i18n.Message{ID: "WindowEdit", Other: "Edit"}
	msgNewTitle       = &i18n.Message{ID: "EditorNewTitle", Other: "New forward"}
	msgEditTitle      = &i18n.Message{ID: "EditorEditTitle", Other: "Edit {{.Name}}"}
	msgCancel         = &i18n.Message{ID: "EditorCancel", Other: "Cancel"}
	msgSave           = &i18n.Message{ID: "EditorSave", Other: "Save"}
	msgDelete         = &i18n.Message{ID: "EditorDelete", Other: "Delete"}
	msgConfirmDelete  = &i18n.Message{ID: "EditorConfirmDelete", Other: "Click again to delete"}
	msgFieldName      = &i18n.Message{ID: "EditorFieldName", Other: "Name"}
	msgFieldGroup     = &i18n.Message{ID: "EditorFieldGroup", Other: "Group"}
	msgFieldContext   = &i18n.Message{ID: "EditorFieldContext", Other: "Context"}
	msgCurrentContext = &i18n.Message{ID: "EditorCurrentContext", Other: "Current context"}
	msgFieldNamespace = &i18n.Message{ID: "EditorFieldNamespace", Other: "Namespace"}
	msgFieldTarget    = &i18n.Message{ID: "EditorFieldTarget", Other: "Target"}
	msgFieldLocal     = &i18n.Message{ID: "EditorFieldLocalPort", Other: "Local port"}
	msgFieldRemote    = &i18n.Message{ID: "EditorFieldRemotePort", Other: "Remote port"}
	msgFieldAddress   = &i18n.Message{ID: "EditorFieldAddress", Other: "Listen address"}
	msgFieldURL       = &i18n.Message{ID: "EditorFieldURL", Other: "URL"}
	msgFieldAutoStart = &i18n.Message{
		ID:    "EditorFieldAutoStart",
		Other: "Start with the application",
	}
	msgDefaultNS = &i18n.Message{ID: "EditorDefaultNamespace", Other: "Default of the context"}
)
