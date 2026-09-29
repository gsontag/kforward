package editor

import (
	"maps"
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/locale"
)

// English, whatever the language of the machine running the tests
var tr = locale.New()

func validForm() Form {
	return Form{
		Name:       "grafana",
		Group:      "Monitoring",
		Context:    "prod",
		Namespace:  "monitoring",
		Target:     "svc/grafana",
		LocalPort:  "3000",
		RemotePort: "http-web",
		URL:        "http://localhost:3000",
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		port intstr.IntOrString
	}{
		{"named remote port", intstr.FromString("http-web")},
		{"numbered remote port", intstr.FromInt32(80)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := config.Forward{
				UUID: "e1564734-b5df-43f4-826f-ee03d6d2931c", Name: "grafana", Group: "Monitoring",
				Context: "prod", Namespace: "monitoring", Target: "svc/grafana",
				LocalPort: 3333, RemotePort: tt.port, Address: "0.0.0.0",
				URL: "http://localhost:3333", AutoStart: true,
			}

			got, problems := FromForward(want).Forward(tr)
			if problems != nil {
				t.Fatalf("unexpected problems: %v", problems)
			}
			check(t, "forward", got, want)
		})
	}
}

// fieldsWithProblems lists the fields of problems, in Field order.
func fieldsWithProblems(problems Problems) []Field {
	return slices.Sorted(maps.Keys(problems))
}

func TestFieldProblems(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*Form)
		field Field
		want  string
	}{
		{"empty name", func(f *Form) { f.Name = "" }, Name, "A name is required"},
		{"blank name", func(f *Form) { f.Name = "   " }, Name, "A name is required"},
		{
			"empty target",
			func(f *Form) { f.Target = "" },
			Target,
			"Expected KIND/NAME, such as svc/grafana: KIND is pod, svc, deploy or sts",
		},
		{"unknown kind", func(f *Form) { f.Target = "cronjob/backup" }, Target, ""},
		{"target without name", func(f *Form) { f.Target = "svc/" }, Target, ""},
		{
			"local port zero",
			func(f *Form) { f.LocalPort = "0" },
			LocalPort,
			"A port number, from 1 to 65535",
		},
		{"local port too large", func(f *Form) { f.LocalPort = "70000" }, LocalPort, ""},
		{"local port with letters", func(f *Form) { f.LocalPort = "3OOO" }, LocalPort, ""},
		{"empty local port", func(f *Form) { f.LocalPort = "" }, LocalPort, ""},
		{"remote port in capitals", func(f *Form) { f.RemotePort = "HTTP" }, RemotePort, ""},
		{"remote port zero", func(f *Form) { f.RemotePort = "0" }, RemotePort, ""},
		{"remote port too large", func(f *Form) { f.RemotePort = "99999999999" }, RemotePort, ""},
		{"empty remote port", func(f *Form) { f.RemotePort = "" }, RemotePort, ""},
		{
			"address",
			func(f *Form) { f.Address = "not-an-ip" },
			Address,
			"An IP address, such as 127.0.0.1, or localhost",
		},
		{
			"relative URL",
			func(f *Form) { f.URL = "grafana" },
			URL,
			"An address starting with http:// or https://",
		},
		{"other scheme", func(f *Form) { f.URL = "ftp://localhost:3000" }, URL, ""},
		{"URL without host", func(f *Form) { f.URL = "http://" }, URL, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := validForm()
			tt.edit(&form)

			got, problems := form.Forward(tr)

			// Only the edited field is reported
			if fields := fieldsWithProblems(problems); !slices.Equal(fields, []Field{tt.field}) {
				t.Fatalf("problems on fields %v, want only %v: %v", fields, tt.field, problems)
			}
			if tt.want != "" {
				check(t, "message", problems[tt.field], tt.want)
			}
			// Nothing half converted can be saved by mistake
			check(t, "forward", got, config.Forward{})
		})
	}
}

func TestAcceptedValues(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Form)
	}{
		{"bare pod name", func(f *Form) { f.Target = "grafana-7d9f" }},
		{"deployment", func(f *Form) { f.Target = "deploy/grafana" }},
		{"lowest port", func(f *Form) { f.LocalPort = "1" }},
		{"highest port", func(f *Form) { f.LocalPort = "65535" }},
		{"localhost", func(f *Form) { f.Address = "localhost" }},
		{"IPv6", func(f *Form) { f.Address = "::1" }},
		{"https", func(f *Form) { f.URL = "https://grafana.example" }},
		{"no URL", func(f *Form) { f.URL = "" }},
		{"no address", func(f *Form) { f.Address = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := validForm()
			tt.edit(&form)
			if _, problems := form.Forward(tr); problems != nil {
				t.Errorf("unexpected problems: %v", problems)
			}
		})
	}
}

func TestSeveralProblems(t *testing.T) {
	form := Form{LocalPort: "abc", RemotePort: "HTTP", Address: "x", URL: "y"}

	_, problems := form.Forward(tr)

	want := []Field{Name, Target, LocalPort, RemotePort, Address, URL}
	if got := fieldsWithProblems(problems); !slices.Equal(got, want) {
		t.Errorf("problems on fields %v, want %v", got, want)
	}
}

func TestTrimmedAndParsed(t *testing.T) {
	form := validForm()
	form.Name, form.Group, form.Target = "  grafana ", " Monitoring\t", " svc/grafana "
	form.LocalPort, form.RemotePort = " 3000 ", " 80 "

	got, problems := form.Forward(tr)
	if problems != nil {
		t.Fatalf("unexpected problems: %v", problems)
	}
	check(t, "name", got.Name, "grafana")
	check(t, "group", got.Group, "Monitoring")
	check(t, "target", got.Target, "svc/grafana")
	check(t, "local port", got.LocalPort, uint16(3000))
	// Digits make a port number, anything else a port name
	check(t, "remote port", got.RemotePort, intstr.FromInt32(80))

	form.RemotePort = "http-web"
	got, _ = form.Forward(tr)
	check(t, "named remote port", got.RemotePort, intstr.FromString("http-web"))
}

// The form must never accept what the configuration would refuse to save.
func TestAcceptedFormPassesConfigValidation(t *testing.T) {
	form := validForm()
	form.UUID = "e1564734-b5df-43f4-826f-ee03d6d2931c"

	f, problems := form.Forward(tr)
	if problems != nil {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if err := f.Validate(); err != nil {
		t.Errorf("config.Validate refuses what the form accepts: %v", err)
	}
}

func TestProblemsInFrench(t *testing.T) {
	fr := locale.New("fr")
	form := validForm()
	form.Name, form.LocalPort = "", "0"

	_, problems := form.Forward(fr)

	check(t, "name", problems[Name], "Un nom est obligatoire")
	check(t, "local port", problems[LocalPort], "Un numéro de port, entre 1 et 65535")
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %+v, want %+v", name, got, want)
	}
}
