package kube

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const kubeconfig = `apiVersion: v1
kind: Config
current-context: prod
clusters:
- name: prod
  cluster:
    server: https://prod.example.invalid
users:
- name: me
  user:
    token: secret
contexts:
- name: prod
  context: {cluster: prod, user: me, namespace: monitoring}
- name: staging
  context: {cluster: prod, user: me}
- name: broken
  context: {cluster: missing, user: me}
`

// An explicit path is required: with an empty one, client-go would read the
// real ~/.kube/config, computed when the package is loaded.
func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(writeKubeconfig(t, kubeconfig))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestContextsSorted(t *testing.T) {
	c := newTestClient(t)

	got, want := c.Contexts(), []string{"broken", "prod", "staging"}
	if !slices.Equal(got, want) {
		t.Errorf("Contexts() = %v, want %v", got, want)
	}
	check(t, "CurrentContext", c.CurrentContext(), "prod")
}

func TestCluster(t *testing.T) {
	tests := []struct {
		name          string
		context       string
		wantContext   string
		wantNamespace string
	}{
		{"current context", "", "prod", "monitoring"},
		{"named context", "prod", "prod", "monitoring"},
		{"default namespace", "staging", "staging", "default"},
	}

	c := newTestClient(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := c.Cluster(tt.context)
			if err != nil {
				t.Fatalf("Cluster(%q): %v", tt.context, err)
			}
			check(t, "Context", cluster.Context, tt.wantContext)
			check(t, "Namespace", cluster.Namespace, tt.wantNamespace)
			check(t, "Host", cluster.Config.Host, "https://prod.example.invalid")
		})
	}
}

func TestClusterErrors(t *testing.T) {
	tests := []struct {
		name    string
		context string
		wantErr string
	}{
		{"missing cluster", "broken", `cluster "missing" not found`},
		{"unknown context", "nope", `unknown context "nope"`},
	}

	c := newTestClient(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := c.Cluster(tt.context)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
			if cluster != nil {
				t.Errorf("got cluster %+v, want nil", cluster)
			}
		})
	}

	// A broken context must not prevent using the others
	if _, err := c.Cluster("prod"); err != nil {
		t.Errorf("Cluster(prod) after a broken context: %v", err)
	}
}

func TestNoCurrentContext(t *testing.T) {
	content := strings.Replace(kubeconfig, "current-context: prod\n", "", 1)
	c, err := NewClient(writeKubeconfig(t, content))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	check(t, "CurrentContext", c.CurrentContext(), "")
	if _, err := c.Cluster(""); err == nil || !strings.Contains(err.Error(), "no current-context") {
		t.Errorf("got error %v, want it to mention the missing current-context", err)
	}
}

func TestExplicitPathMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent")
	if _, err := NewClient(path); err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("got error %v, want it to mention %s", err, path)
	}
}

// KUBECONFIG is read at each call, unlike ~/.kube/config: the merge can be
// tested with files of the test only.
func TestKubeconfigEnvMerge(t *testing.T) {
	extra := writeKubeconfig(t, `apiVersion: v1
kind: Config
contexts:
- name: extra
  context: {cluster: prod, user: me}
`)
	t.Setenv("KUBECONFIG", writeKubeconfig(t, kubeconfig)+string(os.PathListSeparator)+extra)

	c, err := NewClient("")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, want := c.Contexts(), []string{"broken", "extra", "prod", "staging"}
	if !slices.Equal(got, want) {
		t.Errorf("Contexts() = %v, want %v", got, want)
	}
	// The context of the second file uses the cluster of the first one
	if _, err := c.Cluster("extra"); err != nil {
		t.Errorf("Cluster(extra): %v", err)
	}
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
