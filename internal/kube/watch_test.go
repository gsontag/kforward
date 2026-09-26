package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// The informer behind WaitPodGone runs on real time: synctest does not apply.
const watchTimeout = 5 * time.Second

var watched = Endpoint{Namespace: ns, Pod: "grafana-1"}

// startWatch runs WaitPodGone in the background and returns its result channel.
func startWatch(ctx context.Context, cs *fake.Clientset) <-chan error {
	result := make(chan error, 1)
	go func() { result <- WaitPodGone(ctx, cs, watched) }()
	return result
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(watchTimeout):
		t.Fatal("WaitPodGone did not return")
		return nil
	}
}

func TestWaitPodGone(t *testing.T) {
	tests := []struct {
		name    string
		change  func(t *testing.T, cs *fake.Clientset)
		wantMsg string
	}{
		{"deleted", func(t *testing.T, cs *fake.Clientset) {
			if err := cs.CoreV1().Pods(ns).Delete(t.Context(), "grafana-1", metav1.DeleteOptions{}); err != nil {
				t.Fatalf("Delete: %v", err)
			}
		}, "pod grafana-1"},
		{"terminating", func(t *testing.T, cs *fake.Clientset) {
			updatePod(t, cs, newPod("grafana-1", grafanaLabels, terminating))
		}, "pod grafana-1 terminating"},
		{"no longer running", func(t *testing.T, cs *fake.Clientset) {
			pod := newPod("grafana-1", grafanaLabels, ready)
			pod.Status.Phase = corev1.PodFailed
			updatePod(t, cs, pod)
		}, "pod grafana-1 is Failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewClientset(newPod("grafana-1", grafanaLabels, ready))
			result := startWatch(t.Context(), cs)

			tt.change(t, cs)

			err := waitResult(t, result)
			if !errors.Is(err, ErrPodGone) {
				t.Fatalf("got error %v, want ErrPodGone", err)
			}
			// A deletion can be seen by the watch or, if it came first, by the
			// initial list: both messages name the pod
			if !strings.HasPrefix(err.Error(), tt.wantMsg) {
				t.Errorf("error %q should start with %q", err, tt.wantMsg)
			}
		})
	}
}

func updatePod(t *testing.T, cs *fake.Clientset, pod *corev1.Pod) {
	t.Helper()
	if _, err := cs.CoreV1().Pods(ns).Update(t.Context(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestWaitPodGoneAbsentFromStart(t *testing.T) {
	err := waitResult(t, startWatch(t.Context(), fake.NewClientset()))
	if !errors.Is(err, ErrPodGone) {
		t.Errorf("got error %v, want ErrPodGone", err)
	}
}

func TestWaitPodGoneIgnoresOtherPods(t *testing.T) {
	cs := fake.NewClientset(
		newPod("grafana-1", grafanaLabels, ready),
		newPod("grafana-2", grafanaLabels, ready),
	)
	result := startWatch(t.Context(), cs)

	if err := cs.CoreV1().Pods(ns).Delete(t.Context(), "grafana-2", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Proving that nothing happens needs a delay: long enough for the event
	// to reach the watch, short enough to keep the test quick
	select {
	case err := <-result:
		t.Fatalf("WaitPodGone returned %v after another pod was deleted", err)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWaitPodGoneCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	result := startWatch(ctx, fake.NewClientset(newPod("grafana-1", grafanaLabels, ready)))

	cancel()

	if err := waitResult(t, result); err != nil {
		t.Errorf("got error %v, want nil after cancellation", err)
	}
}
