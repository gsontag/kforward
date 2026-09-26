package forward

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
)

// ClusterConnector connects a configured forward through its Kubernetes cluster.
type ClusterConnector struct {
	cluster    *kube.Cluster
	namespace  string
	target     kube.Target
	remotePort intstr.IntOrString
	address    string
	localPort  uint16
}

// NewClusterConnector prepares the connection of f. Its errors come from the
// configuration, such as an unknown context or an invalid target.
func NewClusterConnector(client *kube.Client, f config.Forward) (*ClusterConnector, error) {
	cluster, err := client.Cluster(f.Context)
	if err != nil {
		return nil, err
	}
	target, err := kube.ParseTarget(f.Target)
	if err != nil {
		return nil, err
	}
	namespace := f.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}
	return &ClusterConnector{
		cluster:    cluster,
		namespace:  namespace,
		target:     target,
		remotePort: f.RemotePort,
		address:    f.BindAddress(),
		localPort:  f.LocalPort,
	}, nil
}

// Context returns the name of the kubeconfig context in use.
func (c *ClusterConnector) Context() string {
	return c.cluster.Context
}

// Resolve checks that the local port is free, then finds the pod and port to
// connect to.
func (c *ClusterConnector) Resolve(ctx context.Context) (kube.Endpoint, error) {
	// Fails fast, before any request to the cluster
	if err := checkLocalPort(c.address, c.localPort); err != nil {
		return kube.Endpoint{}, Permanent(err)
	}
	endpoint, err := kube.Resolve(ctx, c.cluster.Clientset, c.namespace, c.target, c.remotePort)
	return endpoint, classify(err)
}

// Forward runs the forward until ctx is cancelled, the connection drops or
// the pod goes away.
func (c *ClusterConnector) Forward(
	ctx context.Context,
	endpoint kube.Endpoint,
	onReady func(),
) error {
	forwardCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// The connection to the API server may outlive the pod: watch it ourselves
	go func() {
		if err := kube.WaitPodGone(forwardCtx, c.cluster.Clientset, endpoint); err != nil {
			cancel(err)
		}
	}()

	err := kube.Forward(forwardCtx, c.cluster, endpoint, c.address, c.localPort, onReady)
	if err == nil && ctx.Err() == nil {
		// Stopped by the pod watch, not by the caller: report why
		err = context.Cause(forwardCtx)
	}
	return classify(err)
}

func checkLocalPort(address string, port uint16) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(address, strconv.Itoa(int(port))))
	if err != nil {
		return fmt.Errorf("local port %d unavailable: %w", port, err)
	}
	return listener.Close()
}

// classify marks the errors that retrying cannot fix as permanent.
func classify(err error) error {
	switch {
	case err == nil:
		return nil
	case apierrors.IsNotFound(err),
		apierrors.IsForbidden(err),
		errors.Is(err, kube.ErrPortNotFound):
		return Permanent(err)
	case apierrors.IsUnauthorized(err):
		// Retrying would run the auth plugin again, possibly opening a browser each time
		return Permanent(err)
	default:
		return err
	}
}
