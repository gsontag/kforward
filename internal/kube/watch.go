package kube

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

// ErrPodGone is returned by WaitPodGone once the pod can no longer serve the forward.
var ErrPodGone = errors.New("pod gone")

// WaitPodGone blocks until the pod of endpoint is deleted, terminating or no
// longer running, and returns an error wrapping ErrPodGone. It returns nil
// when ctx is cancelled first.
func WaitPodGone(ctx context.Context, cs kubernetes.Interface, endpoint Endpoint) error {
	pods := cs.CoreV1().Pods(endpoint.Namespace)
	selector := fields.OneTermEqualSelector("metadata.name", endpoint.Pod).String()
	lw := &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = selector
			return pods.List(ctx, options)
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = selector
			return pods.Watch(ctx, options)
		},
	}

	// The pod may already be gone between the resolution and the watch
	present := func(store cache.Store) (bool, error) {
		_, exists, err := store.GetByKey(endpoint.Namespace + "/" + endpoint.Pod)
		if err == nil && !exists {
			err = fmt.Errorf("pod %s: %w", endpoint.Pod, ErrPodGone)
		}
		return false, err
	}
	gone := func(event watch.Event) (bool, error) {
		pod, ok := event.Object.(*corev1.Pod)
		// The field selector already filters, but not every client honours it
		if !ok || pod.Name != endpoint.Pod {
			return false, nil
		}
		switch {
		case event.Type == watch.Deleted:
			return true, fmt.Errorf("pod %s deleted: %w", pod.Name, ErrPodGone)
		case pod.DeletionTimestamp != nil:
			return true, fmt.Errorf("pod %s terminating: %w", pod.Name, ErrPodGone)
		case pod.Status.Phase != corev1.PodRunning:
			return true, fmt.Errorf("pod %s is %s: %w", pod.Name, pod.Status.Phase, ErrPodGone)
		}
		return false, nil
	}

	// Lets the fake clientset of the tests opt out of the streaming list,
	// which it does not implement; a real clientset keeps it
	watcher := cache.ToListWatcherWithWatchListSemantics(lw, cs)
	_, err := watchtools.UntilWithSync(ctx, watcher, &corev1.Pod{}, present, gone)
	// A cancelled ctx also ends the watch with an error: it is not a disappearance
	if err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
