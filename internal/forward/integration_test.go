//go:build integration

package forward_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gsontag/kforward/internal/kube"
)

// itEnv is the cluster shared by the integration tests of the package.
type itEnv struct {
	client    *kube.Client
	cluster   *kube.Cluster
	namespace string
}

// env is nil when no cluster is configured: the integration tests skip.
var env *itEnv

const setupTimeout = 2 * time.Minute

func TestMain(m *testing.M) {
	path := os.Getenv("KFORWARD_IT_KUBECONFIG")
	if path == "" {
		// Unit tests still run; each integration test skips itself
		m.Run()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	e, err := setup(ctx, path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration setup:", err)
		os.Exit(1)
	}
	env = e
	defer env.teardown()

	m.Run()
}

func setup(ctx context.Context, kubeconfig string) (*itEnv, error) {
	client, err := kube.NewClient(kubeconfig)
	if err != nil {
		return nil, err
	}
	cluster, err := client.Cluster("")
	if err != nil {
		return nil, err
	}

	// A fresh namespace per run: leftovers of a previous run cannot interfere
	ns, err := cluster.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "kforward-it-"},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create namespace: %w", err)
	}
	e := &itEnv{client: client, cluster: cluster, namespace: ns.Name}
	if err := e.createApp(ctx); err != nil {
		e.teardown()
		return nil, err
	}
	return e, nil
}

func (e *itEnv) teardown() {
	// Deletion goes on in the background: no need to wait for it
	err := e.cluster.Clientset.CoreV1().
		Namespaces().
		Delete(context.Background(), e.namespace, metav1.DeleteOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration teardown:", err)
	}
}

// requireCluster returns the shared cluster, or skips the test without one.
func requireCluster(t *testing.T) *itEnv {
	t.Helper()
	if env == nil {
		t.Skip("KFORWARD_IT_KUBECONFIG not set: run make it")
	}
	return env
}

func TestIntegrationClusterReachable(t *testing.T) {
	e := requireCluster(t)

	version, err := e.cluster.Clientset.Discovery().ServerVersion()
	if err != nil {
		t.Fatalf("ServerVersion: %v", err)
	}
	t.Logf(
		"cluster %s, Kubernetes %s, namespace %s",
		e.cluster.Context,
		version.GitVersion,
		e.namespace,
	)
}

func TestIntegrationResolve(t *testing.T) {
	e := requireCluster(t)

	target := kube.Target{Kind: kube.KindService, Name: appName}
	endpoint, err := kube.Resolve(
		t.Context(),
		e.cluster.Clientset,
		e.namespace,
		target,
		intstr.FromString("web"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if endpoint.Port != appPort {
		t.Errorf(
			"Port = %d, want %d: named service port to named container port",
			endpoint.Port,
			appPort,
		)
	}
	if !strings.HasPrefix(endpoint.Pod, appName+"-") {
		t.Errorf("Pod = %q, want a pod of the %s deployment", endpoint.Pod, appName)
	}
}
