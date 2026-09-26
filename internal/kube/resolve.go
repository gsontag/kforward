package kube

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// Endpoint is the pod and container port a forward actually connects to.
type Endpoint struct {
	Namespace string
	Pod       string
	Port      int32
}

// Resolve finds a running pod behind target and the container port designated
// by remote, following the same rules as kubectl port-forward.
func Resolve(
	ctx context.Context,
	cs kubernetes.Interface,
	namespace string,
	target Target,
	remote intstr.IntOrString,
) (Endpoint, error) {
	var (
		pod *corev1.Pod
		svc *corev1.Service
		err error
	)
	if target.Kind == KindPod {
		pod, err = cs.CoreV1().Pods(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err == nil && pod.Status.Phase != corev1.PodRunning {
			err = fmt.Errorf("pod %s is %s, not running", pod.Name, pod.Status.Phase)
		}
	} else {
		var selector labels.Selector
		selector, svc, err = podSelector(ctx, cs, namespace, target)
		if err == nil {
			pod, err = readyPod(ctx, cs, namespace, selector)
		}
	}
	if err != nil {
		return Endpoint{}, fmt.Errorf("%s: %w", target, err)
	}

	port, err := podPort(remote, svc, pod)
	if err != nil {
		return Endpoint{}, fmt.Errorf("%s: %w", target, err)
	}
	return Endpoint{Namespace: namespace, Pod: pod.Name, Port: port}, nil
}

// podSelector returns the label selector of the pods behind target, and the
// service itself when target is one, needed later to translate its ports.
func podSelector(
	ctx context.Context,
	cs kubernetes.Interface,
	namespace string,
	target Target,
) (labels.Selector, *corev1.Service, error) {
	switch target.Kind {
	case KindService:
		svc, err := cs.CoreV1().Services(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		if len(svc.Spec.Selector) == 0 {
			return nil, nil, errors.New("service has no selector")
		}
		return labels.SelectorFromSet(svc.Spec.Selector), svc, nil
	case KindDeployment:
		d, err := cs.AppsV1().Deployments(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
		return selector, nil, err
	case KindStatefulSet:
		s, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, target.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(s.Spec.Selector)
		return selector, nil, err
	default:
		return nil, nil, fmt.Errorf("unsupported kind %q", target.Kind)
	}
}

func readyPod(
	ctx context.Context,
	cs kubernetes.Interface,
	namespace string,
	selector labels.Selector,
) (*corev1.Pod, error) {
	pods, err := cs.CoreV1().
		Pods(namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	for i := range pods.Items {
		if isReady(&pods.Items[i]) {
			return &pods.Items[i], nil
		}
	}
	return nil, fmt.Errorf("no ready pod among %d matching %s", len(pods.Items), selector)
}

func isReady(pod *corev1.Pod) bool {
	// A terminating pod is still Running, but is about to disappear
	if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
