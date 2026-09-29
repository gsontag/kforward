package kube

import (
	"cmp"
	"context"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Short kinds, as typed with kubectl and shown in the configuration examples
var shortKinds = map[Kind]string{
	KindPod:         "pod",
	KindService:     "svc",
	KindDeployment:  "deploy",
	KindStatefulSet: "sts",
}

// Port is a TCP port a forward can target.
type Port struct {
	// Name is empty for an unnamed port.
	Name   string
	Number int32
}

// Namespaces returns the names of the namespaces of the cluster, sorted.
func Namespaces(ctx context.Context, cs kubernetes.Interface) ([]string, error) {
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	slices.Sort(names)
	return names, nil
}

// Targets returns what can be forwarded to in namespace, as KIND/NAME with
// short kinds: services first, then deployments and statefulsets. Pods are
// left out: their generated names do not outlive a restart.
func Targets(ctx context.Context, cs kubernetes.Interface, namespace string) ([]string, error) {
	services, err := cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	deployments, err := cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	statefulSets, err := cs.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var targets []string
	add := func(kind Kind, names []string) {
		slices.Sort(names)
		for _, n := range names {
			targets = append(targets, shortKinds[kind]+"/"+n)
		}
	}
	add(KindService, names(services.Items, func(s corev1.Service) string {
		// Without selector, no pod to forward to
		if len(s.Spec.Selector) == 0 {
			return ""
		}
		return s.Name
	}))
	add(
		KindDeployment,
		names(deployments.Items, func(d appsv1.Deployment) string { return d.Name }),
	)
	add(
		KindStatefulSet,
		names(statefulSets.Items, func(d appsv1.StatefulSet) string { return d.Name }),
	)
	return targets, nil
}

// Ports returns the TCP ports of target: those of the service, or those
// declared by the containers of the pods, sorted by number.
func Ports(
	ctx context.Context,
	cs kubernetes.Interface,
	namespace string,
	target Target,
) ([]Port, error) {
	var ports []Port
	switch target.Kind {
	case KindService:
		svc, err := cs.CoreV1().Services(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		for _, p := range svc.Spec.Ports {
			if p.Protocol == "" || p.Protocol == corev1.ProtocolTCP {
				ports = append(ports, Port{Name: p.Name, Number: p.Port})
			}
		}
	case KindDeployment:
		d, err := cs.AppsV1().Deployments(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ports = containerPorts(d.Spec.Template.Spec)
	case KindStatefulSet:
		s, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ports = containerPorts(s.Spec.Template.Spec)
	case KindPod:
		pod, err := cs.CoreV1().Pods(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ports = containerPorts(pod.Spec)
	}
	slices.SortFunc(ports, func(a, b Port) int { return cmp.Compare(a.Number, b.Number) })
	return ports, nil
}

func names[T any](items []T, name func(T) string) []string {
	result := make([]string, 0, len(items))
	for _, it := range items {
		if n := name(it); n != "" {
			result = append(result, n)
		}
	}
	return result
}

func containerPorts(spec corev1.PodSpec) []Port {
	var ports []Port
	for _, c := range spec.Containers {
		for _, p := range c.Ports {
			if p.Protocol == "" || p.Protocol == corev1.ProtocolTCP {
				ports = append(ports, Port{Name: p.Name, Number: p.ContainerPort})
			}
		}
	}
	return ports
}
