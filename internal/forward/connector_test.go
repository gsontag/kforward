package forward

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
)

func TestClassify(t *testing.T) {
	pods := corev1.Resource("pods")
	tests := []struct {
		name          string
		err           error
		wantPermanent bool
	}{
		{"not found", apierrors.NewNotFound(pods, "grafana"), true},
		{"forbidden", apierrors.NewForbidden(pods, "grafana", errors.New("rbac")), true},
		{"unauthorized", apierrors.NewUnauthorized("token expired"), true},
		{"port not found", fmt.Errorf("no TCP service port 81: %w", kube.ErrPortNotFound), true},
		{"wrapped not found", fmt.Errorf("service/grafana: %w", apierrors.NewNotFound(pods, "grafana")), true},
		{"pod gone", fmt.Errorf("pod grafana-1 deleted: %w", kube.ErrPodGone), false},
		{"no ready pod", errors.New("no ready pod among 2 matching app=grafana"), false},
		{"server error", apierrors.NewInternalError(errors.New("etcd")), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.err)
			check(t, "IsPermanent", IsPermanent(got), tt.wantPermanent)
			// Classifying only adds a mark: the original error stays reachable
			check(t, "errors.Is", errors.Is(got, tt.err), true)
		})
	}

	if classify(nil) != nil {
		t.Error("classify(nil) should be nil")
	}
}

// freePort returns a local port that nothing listens on.
func freePort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return uint16(port)
}

// busyPort returns a local port kept busy until the end of the test.
func busyPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return uint16(listener.Addr().(*net.TCPAddr).Port)
}

func TestCheckLocalPort(t *testing.T) {
	if err := checkLocalPort("127.0.0.1", freePort(t)); err != nil {
		t.Errorf("free port: %v", err)
	}

	port := busyPort(t)
	err := checkLocalPort("127.0.0.1", port)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("local port %d unavailable", port)) {
		t.Errorf("busy port: got error %v", err)
	}
}

// testConnector builds a connector on a fake cluster; requests counts the
// requests it receives.
func testConnector(t *testing.T, localPort uint16, objects ...runtime.Object) (*ClusterConnector, *int) {
	t.Helper()
	cs := fake.NewClientset(objects...)
	requests := 0
	// Not handled: the request goes on to the objects of the fake clientset
	cs.PrependReactor("*", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		requests++
		return false, nil, nil
	})
	return &ClusterConnector{
		cluster:    &kube.Cluster{Context: "test", Namespace: "default", Clientset: cs},
		namespace:  "monitoring",
		target:     kube.Target{Kind: kube.KindService, Name: "grafana"},
		remotePort: intstr.FromInt32(80),
		address:    "127.0.0.1",
		localPort:  localPort,
	}, &requests
}

func TestResolvePortBusy(t *testing.T) {
	c, requests := testConnector(t, busyPort(t))

	_, err := c.Resolve(t.Context())

	check(t, "IsPermanent", IsPermanent(err), true)
	// The whole point of the check: no round trip to the cluster
	check(t, "requests", *requests, 0)
}

func TestResolveServiceMissing(t *testing.T) {
	c, requests := testConnector(t, freePort(t))

	_, err := c.Resolve(t.Context())

	check(t, "IsPermanent", IsPermanent(err), true)
	check(t, "IsNotFound", apierrors.IsNotFound(err), true)
	check(t, "requests", *requests, 1)
}

func TestResolveNoReadyPodIsTransient(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "grafana", Namespace: "monitoring"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "grafana"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	c, _ := testConnector(t, freePort(t), svc)

	_, err := c.Resolve(t.Context())

	if err == nil || IsPermanent(err) {
		t.Errorf("got error %v, want a transient error: a rollout may bring a pod", err)
	}
}

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
`

func testClient(t *testing.T) *kube.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	client, err := kube.NewClient(path)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestNewClusterConnector(t *testing.T) {
	client := testClient(t)
	base := config.Forward{Name: "grafana", Target: "svc/grafana", LocalPort: 3000, RemotePort: intstr.FromInt32(80)}

	c, err := NewClusterConnector(client, base)
	if err != nil {
		t.Fatalf("NewClusterConnector: %v", err)
	}
	check(t, "Context", c.Context(), "prod")
	check(t, "namespace from context", c.namespace, "monitoring")
	check(t, "address", c.address, "127.0.0.1")

	explicit := base
	explicit.Namespace = "other"
	c, err = NewClusterConnector(client, explicit)
	if err != nil {
		t.Fatalf("NewClusterConnector: %v", err)
	}
	check(t, "explicit namespace", c.namespace, "other")
}

func TestNewClusterConnectorErrors(t *testing.T) {
	tests := []struct {
		name    string
		forward config.Forward
		wantErr string
	}{
		{"unknown context", config.Forward{Context: "nope", Target: "svc/grafana"}, `unknown context "nope"`},
		{"invalid target", config.Forward{Target: "cronjob/backup"}, `unsupported kind "cronjob"`},
	}

	client := testClient(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClusterConnector(client, tt.forward)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
			if c != nil {
				t.Error("got a connector, want nil")
			}
		})
	}
}
