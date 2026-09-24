package config

import (
	"bytes"
	"encoding/json"
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
		{"numéro", intstr.FromInt32(80), `"remote-port":80`},
		{"port nommé", intstr.FromString("http-web"), `"remote-port":"http-web"`},
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
	// Une slice nil sortirait en null, que la popup devrait ensuite gérer
	check(t, "DefaultConfig JSON", string(data), `{"forwards":[]}`)
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
