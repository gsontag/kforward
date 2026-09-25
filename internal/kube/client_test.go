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

// Un chemin explicite est indispensable : avec un chemin vide, client-go lirait
// le vrai ~/.kube/config, calculé au chargement du package.
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
		{"contexte courant", "", "prod", "monitoring"},
		{"contexte nommé", "prod", "prod", "monitoring"},
		{"namespace par défaut", "staging", "staging", "default"},
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
		{"cluster manquant", "broken", `cluster "missing" not found`},
		{"contexte inconnu", "nope", `unknown context "nope"`},
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

	// Le contexte cassé ne doit pas empêcher d'utiliser les autres
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

// KUBECONFIG est relu à chaque appel, contrairement à ~/.kube/config :
// on peut donc tester la fusion en ne pointant que vers des fichiers du test.
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
	// Le contexte du second fichier utilise le cluster défini dans le premier
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
