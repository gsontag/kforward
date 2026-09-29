package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
)

var configFileContent = []byte(`
{
	"kubeconfig": "somefile",
	"forwards": [
		{
			"uuid": "1",
			"name": "intport",
			"context": "ctx",
			"namespace": "ns",
			"target": "svc/service",
			"local-port": 1000,
			"remote-port": 3000,
			"address": "0.0.0.0",
			"auto-start": true
		},
		{
			"uuid": "2",
			"name": "strport",
			"context": "ctx",
			"namespace": "ns",
			"target": "svc/service",
			"local-port": 1000,
			"remote-port": "http",
			"url": "http://localhost:1000",
			"auto-start": false
		}
	]
}`)

func TestUnmarshalConfig(t *testing.T) {
	c := Config{}
	if err := json.Unmarshal(configFileContent, &c); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	check(t, "Kubeconfig", c.Kubeconfig, "somefile")

	// Fatal : la suite indexe c.Forwards et paniquerait
	if len(c.Forwards) != 2 {
		t.Fatalf("got %d forwards, want 2", len(c.Forwards))
	}

	fw := c.Forwards[0]

	check(t, "RemotePort type", fw.RemotePort.Type, intstr.Int)
	check(t, "RemotePort value", fw.RemotePort.IntVal, 3000)
	check(t, "BindAddress", fw.BindAddress(), "0.0.0.0")
	check(t, "AutoStart", fw.AutoStart, true)

	fw = c.Forwards[1]

	check(t, "RemotePort type", fw.RemotePort.Type, intstr.String)
	check(t, "RemotePort value", fw.RemotePort.StrVal, "http")
	check(t, "BindAddress", fw.BindAddress(), "127.0.0.1")
	check(t, "URL", fw.URL, "http://localhost:1000")
}

func TestMarshalOmitsDefaults(t *testing.T) {
	fw := Forward{
		UUID:       "1",
		Name:       "minimal",
		Target:     "svc/service",
		LocalPort:  1000,
		RemotePort: intstr.FromInt32(3000),
	}
	data, err := json.Marshal(fw)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for _, key := range []string{"group", "address", "url", "namespace", "context"} {
		if bytes.Contains(data, []byte(`"`+key+`"`)) {
			t.Errorf("key %q should be omitted: %s", key, data)
		}
	}
}

func TestMarshalRemotePort(t *testing.T) {
	tests := []struct {
		name string
		port intstr.IntOrString
		want string
	}{
		{"number", intstr.FromInt32(80), `"remote-port":80`},
		{"named port", intstr.FromString("http-web"), `"remote-port":"http-web"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(Forward{RemotePort: tt.port})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !bytes.Contains(data, []byte(tt.want)) {
				t.Errorf("got %s, want it to contain %s", data, tt.want)
			}
		})
	}
}

func TestDefaultConfigMarshal(t *testing.T) {
	data, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// A nil slice would be written as null, which the popup would have to handle
	check(t, "DefaultConfig JSON", string(data), `{"forwards":[]}`)
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func validForward() Forward {
	return Forward{
		UUID: "1", Name: "grafana", Target: "svc/grafana",
		LocalPort: 3000, RemotePort: intstr.FromInt32(80),
	}
}

func TestForwardValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Forward)
		wantErr string // empty = must be valid
	}{
		{"valid", func(*Forward) {}, ""},
		{"no name", func(f *Forward) { f.Name = "  " }, "name is required"},
		{"local port 0", func(f *Forward) { f.LocalPort = 0 }, "local-port is required"},
		{"no target", func(f *Forward) { f.Target = "" }, "target is required"},
		{
			"remote port missing",
			func(f *Forward) { f.RemotePort = intstr.IntOrString{} },
			"between 1 and 65535",
		},
		{
			"remote port too large",
			func(f *Forward) { f.RemotePort = intstr.FromInt32(70000) },
			"between 1 and 65535",
		},
		{"named port", func(f *Forward) { f.RemotePort = intstr.FromString("http-web") }, ""},
		{
			"named port in capitals",
			func(f *Forward) { f.RemotePort = intstr.FromString("HTTP") },
			"remote-port:",
		},
		{
			"named port too long",
			func(f *Forward) { f.RemotePort = intstr.FromString("un-nom-bien-trop-long") },
			"remote-port:",
		},
		{
			"empty named port",
			func(f *Forward) { f.RemotePort = intstr.FromString("") },
			"remote-port:",
		},
		{"localhost address", func(f *Forward) { f.Address = "localhost" }, ""},
		{"any address", func(f *Forward) { f.Address = "0.0.0.0" }, ""},
		{"IPv6 address", func(f *Forward) { f.Address = "::1" }, ""},
		{"invalid address", func(f *Forward) { f.Address = "not-an-ip" }, `address "not-an-ip"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validForward()
			tt.mutate(&f)
			err := f.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("got error %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	second := validForward()
	second.UUID, second.Name = "2", "prometheus"

	tests := []struct {
		name     string
		forwards []Forward
		wantErrs []string // empty = must be valid
	}{
		{"empty", nil, nil},
		{"two distinct forwards", []Forward{validForward(), second}, nil},
		{
			"duplicate uuid",
			[]Forward{validForward(), validForward()},
			[]string{`forward #2 (grafana): duplicate uuid "1"`},
		},
		{
			"empty uuid",
			[]Forward{
				{
					Name:       "grafana",
					Target:     "svc/grafana",
					LocalPort:  3000,
					RemotePort: intstr.FromInt32(80),
				},
			},
			[]string{"forward #1 (grafana): uuid is required"},
		},
		{
			// No error may be lost, and each one has its prefix
			"several errors",
			[]Forward{validForward(), {UUID: "1", RemotePort: intstr.FromInt32(80)}},
			[]string{
				`forward #2: duplicate uuid "1"`,
				"forward #2: name is required",
				"forward #2: target is required",
				"forward #2: local-port is required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Config{Forwards: tt.forwards}).Validate()

			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("got error %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("got nil error, want %d errors", len(tt.wantErrs))
			}
			for _, want := range tt.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error should contain %q, got:\n%v", want, err)
				}
			}
		})
	}
}
