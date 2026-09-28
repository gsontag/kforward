// Package editor turns the fields of the edit dialog into a forward: it
// parses what the user typed and explains, field by field, what is wrong.
package editor

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
	"gsontag.fr/kforward/internal/locale"
)

// Field names a field of the form, to show its problem next to it.
type Field int

// Fields of the form that can hold a problem.
const (
	Name Field = iota
	Target
	LocalPort
	RemotePort
	Address
	URL
)

// Form holds the fields as typed: text, so that a wrong entry can be shown
// back to the user unchanged.
type Form struct {
	// UUID is empty for a new forward.
	UUID                                    string
	Name, Group, Context, Namespace, Target string
	LocalPort, RemotePort, Address, URL     string
	AutoStart                               bool
}

// Problems explains, in the language of the user, what is wrong per field.
type Problems map[Field]string

// FromForward fills a form with an existing forward.
func FromForward(f config.Forward) Form {
	return Form{
		UUID:       f.UUID,
		Name:       f.Name,
		Group:      f.Group,
		Context:    f.Context,
		Namespace:  f.Namespace,
		Target:     f.Target,
		LocalPort:  strconv.Itoa(int(f.LocalPort)),
		RemotePort: f.RemotePort.String(),
		Address:    f.Address,
		URL:        f.URL,
		AutoStart:  f.AutoStart,
	}
}

// Forward returns the forward described by the form, or the problems that
// prevent it; the texts are trimmed.
func (form Form) Forward(tr *locale.Translator) (config.Forward, Problems) {
	f := config.Forward{
		UUID:      form.UUID,
		Name:      strings.TrimSpace(form.Name),
		Group:     strings.TrimSpace(form.Group),
		Context:   strings.TrimSpace(form.Context),
		Namespace: strings.TrimSpace(form.Namespace),
		Target:    strings.TrimSpace(form.Target),
		Address:   strings.TrimSpace(form.Address),
		URL:       strings.TrimSpace(form.URL),
		AutoStart: form.AutoStart,
	}
	problems := Problems{}

	if f.Name == "" {
		problems[Name] = tr.T(msgNameRequired, nil)
	}
	if _, err := kube.ParseTarget(f.Target); err != nil {
		problems[Target] = tr.T(msgTarget, nil)
	}
	if port, ok := parsePort(form.LocalPort); ok {
		f.LocalPort = uint16(port)
	} else {
		problems[LocalPort] = tr.T(msgLocalPort, nil)
	}
	if port, ok := parseRemotePort(form.RemotePort); ok {
		f.RemotePort = port
	} else {
		problems[RemotePort] = tr.T(msgRemotePort, nil)
	}
	if f.Address != "" && f.Address != "localhost" && net.ParseIP(f.Address) == nil {
		problems[Address] = tr.T(msgAddress, nil)
	}
	if f.URL != "" && !validURL(f.URL) {
		problems[URL] = tr.T(msgURL, nil)
	}

	if len(problems) > 0 {
		return config.Forward{}, problems
	}
	return f, nil
}

func parsePort(s string) (int, bool) {
	port, err := strconv.Atoi(strings.TrimSpace(s))
	return port, err == nil && port >= 1 && port <= 65535
}

// parseRemotePort reads a number as a port number, anything else as a name.
func parseRemotePort(s string) (intstr.IntOrString, bool) {
	s = strings.TrimSpace(s)
	if port, err := strconv.Atoi(s); err == nil {
		return intstr.FromInt32(int32(port)), port >= 1 && port <= 65535
	}
	return intstr.FromString(s), len(validation.IsValidPortName(s)) == 0
}

func validURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
