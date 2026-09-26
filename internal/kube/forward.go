package kube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/streaming/pkg/httpstream"
)

// Forward listens on address:localPort and forwards every connection to the
// endpoint. It blocks until ctx is cancelled, then returns nil, or until the
// forward fails. onReady is called, from another goroutine, once the local
// port accepts connections.
func Forward(
	ctx context.Context,
	cluster *Cluster,
	endpoint Endpoint,
	address string,
	localPort uint16,
	onReady func(),
) error {
	// The dial ignores ctx: do not open a connection that is already unwanted
	if ctx.Err() != nil {
		return nil
	}
	dialer, err := newDialer(cluster, endpoint)
	if err != nil {
		return err
	}

	// errOut is only written before ForwardPorts returns: no locking needed
	var errOut strings.Builder
	ready := make(chan struct{})
	ports := []string{fmt.Sprintf("%d:%d", localPort, endpoint.Port)}
	pf, err := portforward.NewOnAddressesForStreamingWithContext(
		ctx, dialer, []string{address}, ports, ready, io.Discard, &errOut,
	)
	if err != nil {
		return err
	}

	// ForwardPorts only signals readiness by closing the ready channel
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ready:
			onReady()
		case <-done:
		}
	}()
	if err := pf.ForwardPorts(); err != nil {
		if detail := strings.TrimSpace(errOut.String()); detail != "" {
			return fmt.Errorf("%w: %s", err, detail)
		}
		return err
	}
	return nil
}

// newDialer opens the stream to the portforward subresource of the pod,
// preferring WebSocket and falling back to SPDY, like kubectl.
func newDialer(cluster *Cluster, endpoint Endpoint) (httpstream.Dialer, error) {
	url := cluster.Clientset.CoreV1().
		RESTClient().
		Post().
		Resource("pods").
		Namespace(endpoint.Namespace).
		Name(endpoint.Pod).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(cluster.Config)
	if err != nil {
		return nil, err
	}
	spdyDialer := spdy.NewDialerForStreaming(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		url,
	)

	websocketDialer, err := portforward.NewSPDYOverWebsocketDialerForStreaming(url, cluster.Config)
	if err != nil {
		return nil, err
	}
	return portforward.NewFallbackDialerForStreaming(
		websocketDialer,
		spdyDialer,
		func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		},
	), nil
}
