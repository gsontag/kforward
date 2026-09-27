//go:build integration

package forward_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	appName = "web"
	appPort = 8080
)

var appLabels = map[string]string{"app": appName}

// appImage must already be in the cluster: make it-cluster loads it.
func appImage() string {
	if image := os.Getenv("KFORWARD_IT_IMAGE"); image != "" {
		return image
	}
	return "busybox:stable"
}

// createApp deploys an HTTP server answering with the name of its pod, behind
// a service whose named targetPort exercises the port translation.
func (e *itEnv) createApp(ctx context.Context) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: appLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: appLabels},
				Spec: corev1.PodSpec{
					// httpd runs as PID 1 and ignores SIGTERM: without this, each
					// pod deletion would wait the default 30 s
					TerminationGracePeriodSeconds: new(int64(1)),
					Containers: []corev1.Container{{
						Name:            appName,
						Image:           appImage(),
						ImagePullPolicy: corev1.PullNever,
						Command: []string{
							"sh",
							"-c",
							"mkdir -p /www && hostname > /www/index.html && exec httpd -f -p 8080 -h /www",
						},
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: appPort},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/",
									Port: intstr.FromString("http"),
								},
							},
							PeriodSeconds: 1,
						},
					}},
				},
			},
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: appName},
		Spec: corev1.ServiceSpec{
			Selector: appLabels,
			Ports: []corev1.ServicePort{
				{Name: "web", Port: 80, TargetPort: intstr.FromString("http")},
			},
		},
	}

	cs := e.cluster.Clientset
	if _, err := cs.AppsV1().Deployments(e.namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}
	if _, err := cs.CoreV1().Services(e.namespace).Create(ctx, service, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	return e.waitAppReady(ctx)
}

func (e *itEnv) waitAppReady(ctx context.Context) error {
	deployments := e.cluster.Clientset.AppsV1().Deployments(e.namespace)
	err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true,
		func(ctx context.Context) (bool, error) {
			d, err := deployments.Get(ctx, appName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			// Status describes the current spec only once the controller has seen it
			return d.Status.ObservedGeneration >= d.Generation &&
				d.Status.ReadyReplicas == *d.Spec.Replicas &&
				d.Status.Replicas == *d.Spec.Replicas, nil
		},
	)
	if err != nil {
		return fmt.Errorf("wait for deployment %s: %w", appName, err)
	}
	return nil
}

// deletePod deletes a pod of the test app; its deployment replaces it.
func (e *itEnv) deletePod(t *testing.T, name string) {
	t.Helper()
	err := e.cluster.Clientset.CoreV1().
		Pods(e.namespace).
		Delete(t.Context(), name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("delete pod %s: %v", name, err)
	}
	// The next test must find the app as it was: wait for the replacement pod
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), stateTimeout)
		defer cancel()
		if err := e.waitAppReady(ctx); err != nil {
			t.Errorf("app not ready again after the pod deletion: %v", err)
		}
	})
}

// scaleApp sets the replicas of the test app, and restores a single ready
// replica at the end of the test.
func (e *itEnv) scaleApp(t *testing.T, replicas int32) {
	t.Helper()
	e.setReplicas(t.Context(), t, replicas)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), stateTimeout)
		defer cancel()
		e.setReplicas(ctx, t, 1)
		if err := e.waitAppReady(ctx); err != nil {
			t.Errorf("app not ready again after scaling: %v", err)
		}
	})
}

func (e *itEnv) setReplicas(ctx context.Context, t *testing.T, replicas int32) {
	t.Helper()
	deployments := e.cluster.Clientset.AppsV1().Deployments(e.namespace)
	// The scale subresource changes the replicas without touching the rest of the spec
	scale, err := deployments.GetScale(ctx, appName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get scale: %v", err)
	}
	scale.Spec.Replicas = replicas
	if _, err := deployments.UpdateScale(ctx, appName, scale, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update scale: %v", err)
	}
}
