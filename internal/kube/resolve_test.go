package kube

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const ns = "monitoring"

type podState int

const (
	ready podState = iota
	notReady
	noCondition
	pending
	terminating
)

func newPod(name string, labels map[string]string, state podState) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: "app", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 3000}}},
		}},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
	switch state {
	case ready:
	case notReady:
		pod.Status.Conditions[0].Status = corev1.ConditionFalse
	case noCondition:
		// Running, but the kubelet has not reported readiness yet
		pod.Status.Conditions = nil
	case pending:
		pod.Status.Phase = corev1.PodPending
		pod.Status.Conditions = nil
	case terminating:
		// The fake clientset stores objects as given: no finalizer is needed
		pod.DeletionTimestamp = &metav1.Time{}
	}
	return pod
}

var grafanaLabels = map[string]string{"app": "grafana"}

func grafanaService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "grafana", Namespace: ns},
		Spec: corev1.ServiceSpec{
			Selector: grafanaLabels,
			Ports:    []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromString("web")}},
		},
	}
}

func resolve(t *testing.T, target string, remote intstr.IntOrString, objects ...runtime.Object) (Endpoint, error) {
	t.Helper()
	parsed, err := ParseTarget(target)
	if err != nil {
		t.Fatalf("ParseTarget(%q): %v", target, err)
	}
	return Resolve(t.Context(), fake.NewClientset(objects...), ns, parsed, remote)
}

func TestResolve(t *testing.T) {
	labelSelector := &metav1.LabelSelector{MatchLabels: grafanaLabels}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "grafana", Namespace: ns},
		Spec:       appsv1.DeploymentSpec{Selector: labelSelector},
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "grafana", Namespace: ns},
		Spec: appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"grafana", "other"}},
			},
		}},
	}

	tests := []struct {
		name    string
		target  string
		remote  intstr.IntOrString
		objects []runtime.Object
		wantPod string
	}{
		{
			"service by port number",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{grafanaService(), newPod("grafana-1", grafanaLabels, ready)},
			"grafana-1",
		},
		{
			"service by port name",
			"svc/grafana", intstr.FromString("http"),
			[]runtime.Object{grafanaService(), newPod("grafana-1", grafanaLabels, ready)},
			"grafana-1",
		},
		{
			"pods of other labels ignored",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{
				grafanaService(),
				newPod("aaa-other", map[string]string{"app": "prometheus"}, ready),
				newPod("grafana-1", grafanaLabels, ready),
			},
			"grafana-1",
		},
		{
			"ready pod preferred",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{
				grafanaService(),
				newPod("grafana-1", grafanaLabels, notReady),
				newPod("grafana-2", grafanaLabels, terminating),
				newPod("grafana-3", grafanaLabels, ready),
			},
			"grafana-3",
		},
		{
			"deployment, named port",
			"deploy/grafana", intstr.FromString("web"),
			[]runtime.Object{deployment, newPod("grafana-1", grafanaLabels, ready)},
			"grafana-1",
		},
		{
			"statefulset, selector expression",
			"sts/grafana", intstr.FromString("web"),
			[]runtime.Object{statefulSet, newPod("grafana-0", grafanaLabels, ready)},
			"grafana-0",
		},
		{
			// Like kubectl: a pod targeted directly only has to run
			"pod running but not ready",
			"pod/grafana-1", intstr.FromInt32(3000),
			[]runtime.Object{newPod("grafana-1", grafanaLabels, notReady)},
			"grafana-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolve(t, tt.target, tt.remote, tt.objects...)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			check(t, "Endpoint", got, Endpoint{Namespace: ns, Pod: tt.wantPod, Port: 3000})
		})
	}
}

func TestResolveErrors(t *testing.T) {
	serviceWithoutSelector := grafanaService()
	serviceWithoutSelector.Spec.Selector = nil

	otherNamespace := grafanaService()
	otherNamespace.Namespace = "elsewhere"

	tests := []struct {
		name    string
		target  string
		remote  intstr.IntOrString
		objects []runtime.Object
		wantErr string
	}{
		{
			"no ready pod",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{
				grafanaService(),
				newPod("grafana-1", grafanaLabels, notReady),
				newPod("grafana-2", grafanaLabels, terminating),
				newPod("grafana-3", grafanaLabels, noCondition),
			},
			"service/grafana: no ready pod among 3 matching app=grafana",
		},
		{
			"no matching pod",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{grafanaService()},
			"no ready pod among 0 matching app=grafana",
		},
		{
			"service without selector",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{serviceWithoutSelector},
			"service/grafana: service has no selector",
		},
		{
			"service in another namespace",
			"svc/grafana", intstr.FromInt32(80),
			[]runtime.Object{otherNamespace},
			`services "grafana" not found`,
		},
		{
			"pending pod",
			"pod/grafana-1", intstr.FromInt32(3000),
			[]runtime.Object{newPod("grafana-1", grafanaLabels, pending)},
			"pod grafana-1 is Pending, not running",
		},
		{
			"unknown service port",
			"svc/grafana", intstr.FromInt32(81),
			[]runtime.Object{grafanaService(), newPod("grafana-1", grafanaLabels, ready)},
			"service/grafana: no TCP service port 81",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolve(t, tt.target, tt.remote, tt.objects...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
			check(t, "Endpoint", got, Endpoint{})
		})
	}
}

// The target prefix must not hide the API error from callers.
func TestResolveKeepsNotFound(t *testing.T) {
	for _, target := range []string{"svc/grafana", "deploy/grafana", "sts/grafana", "pod/grafana"} {
		t.Run(target, func(t *testing.T) {
			_, err := resolve(t, target, intstr.FromInt32(80))
			if !apierrors.IsNotFound(err) {
				t.Errorf("got error %v, want a NotFound error", err)
			}
		})
	}
}

// Guards against a kind added to kindAliases but not to podSelector.
func TestResolveUnsupportedKind(t *testing.T) {
	_, err := Resolve(t.Context(), fake.NewClientset(), ns, Target{Kind: "cronjob", Name: "x"}, intstr.FromInt32(80))
	if err == nil || !strings.Contains(err.Error(), `unsupported kind "cronjob"`) {
		t.Errorf("got error %v, want an unsupported kind error", err)
	}
}

func TestEveryAliasIsResolvable(t *testing.T) {
	for alias, kind := range kindAliases {
		if kind == KindPod {
			continue
		}
		t.Run(alias, func(t *testing.T) {
			_, err := Resolve(t.Context(), fake.NewClientset(), ns, Target{Kind: kind, Name: "x"}, intstr.FromInt32(80))
			if err != nil && strings.Contains(err.Error(), "unsupported kind") {
				t.Errorf("kind %q is parsed but not handled by podSelector", kind)
			}
		})
	}
}

// A reactor intercepts the requests of the fake clientset: here, it simulates
// an API server refusing to list pods.
func TestResolveListError(t *testing.T) {
	cs := fake.NewClientset(grafanaService())
	cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(corev1.Resource("pods"), "", nil)
	})

	target := Target{Kind: KindService, Name: "grafana"}
	_, err := Resolve(t.Context(), cs, ns, target, intstr.FromInt32(80))
	if !apierrors.IsForbidden(err) {
		t.Errorf("got error %v, want a Forbidden error", err)
	}
}
