package editor

import (
	"context"

	"github.com/gsontag/kforward/internal/kube"
)

// ClusterSource answers the suggestions from the clusters of a kubeconfig.
type ClusterSource struct {
	Client *kube.Client
}

// Namespaces lists the namespaces of the cluster of kubeContext.
func (s ClusterSource) Namespaces(ctx context.Context, kubeContext string) ([]string, error) {
	cluster, err := s.Client.Cluster(kubeContext)
	if err != nil {
		return nil, err
	}
	return kube.Namespaces(ctx, cluster.Clientset)
}

// Targets lists the targets of namespace, the default one of the context
// when empty.
func (s ClusterSource) Targets(
	ctx context.Context,
	kubeContext, namespace string,
) ([]string, error) {
	cluster, err := s.Client.Cluster(kubeContext)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		namespace = cluster.Namespace
	}
	return kube.Targets(ctx, cluster.Clientset, namespace)
}

// Ports lists the ports of target; a target still being typed has none.
func (s ClusterSource) Ports(
	ctx context.Context,
	kubeContext, namespace, target string,
) ([]kube.Port, error) {
	parsed, err := kube.ParseTarget(target)
	if err != nil {
		return nil, nil //nolint:nilerr // not an error: the user is still typing
	}
	cluster, err := s.Client.Cluster(kubeContext)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		namespace = cluster.Namespace
	}
	return kube.Ports(ctx, cluster.Clientset, namespace, parsed)
}
