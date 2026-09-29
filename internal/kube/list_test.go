package kube

import (
	"slices"
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

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func TestNamespaces(t *testing.T) {
	cs := fake.NewClientset(namespace("monitoring"), namespace("default"), namespace("kube-system"))

	got, err := Namespaces(t.Context(), cs)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if want := []string{"default", "kube-system", "monitoring"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Listing the namespaces needs a cluster-wide right that many users lack.
func TestNamespacesForbidden(t *testing.T) {
	cs := fake.NewClientset()
	cs.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(corev1.Resource("namespaces"), "", nil)
	})

	if _, err := Namespaces(t.Context(), cs); !apierrors.IsForbidden(err) {
		t.Errorf("got error %v, want a Forbidden error", err)
	}
}

func deployment(name, namespace string, spec corev1.PodSpec) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: spec}},
	}
}

func statefulSet(name, namespace string, spec corev1.PodSpec) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: spec}},
	}
}

func service(
	name, namespace string,
	selector map[string]string,
	ports ...corev1.ServicePort,
) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       corev1.ServiceSpec{Selector: selector, Ports: ports},
	}
}

func TestTargets(t *testing.T) {
	cs := fake.NewClientset(
		service("prometheus", ns, grafanaLabels),
		service("grafana", ns, grafanaLabels),
		// Endpoints set by hand, no pod behind: not proposed
		service("external-db", ns, nil),
		deployment("grafana", ns, corev1.PodSpec{}),
		statefulSet("loki", ns, corev1.PodSpec{}),
		statefulSet("alertmanager", ns, corev1.PodSpec{}),
		// Another namespace: not proposed
		service("keycloak", "auth", grafanaLabels),
		newPod("grafana-1", grafanaLabels, ready),
	)

	got, err := Targets(t.Context(), cs, ns)
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	want := []string{
		"svc/grafana",
		"svc/prometheus",
		"deploy/grafana",
		"sts/alertmanager",
		"sts/loki",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Whatever is proposed must be accepted by the form
	for _, target := range got {
		if _, err := ParseTarget(target); err != nil {
			t.Errorf("proposed target %q is refused: %v", target, err)
		}
	}
}

func TestTargetsEmptyNamespace(t *testing.T) {
	got, err := Targets(t.Context(), fake.NewClientset(), "empty")
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	check(t, "targets", len(got), 0)
}

func containers(ports ...[]corev1.ContainerPort) corev1.PodSpec {
	spec := corev1.PodSpec{}
	for i, p := range ports {
		spec.Containers = append(
			spec.Containers,
			corev1.Container{Name: string(rune('a' + i)), Ports: p},
		)
	}
	return spec
}

func TestPorts(t *testing.T) {
	// A sidecar and the application: the ports of both containers count
	spec := containers(
		[]corev1.ContainerPort{{Name: "metrics", ContainerPort: 9090}},
		[]corev1.ContainerPort{
			{Name: "http", ContainerPort: 3000},
			{Name: "syslog", ContainerPort: 514, Protocol: corev1.ProtocolUDP},
			{ContainerPort: 22},
		},
	)
	pod := newPod("grafana-1", grafanaLabels, ready)
	pod.Spec = spec
	cs := fake.NewClientset(
		service("grafana", ns, grafanaLabels,
			corev1.ServicePort{Name: "web", Port: 80, TargetPort: intstr.FromString("http")},
			corev1.ServicePort{Name: "dns", Port: 53, Protocol: corev1.ProtocolUDP},
			corev1.ServicePort{Name: "metrics", Port: 9090, Protocol: corev1.ProtocolTCP},
		),
		deployment("grafana", ns, spec),
		statefulSet("grafana", ns, spec),
		pod,
	)
	fromContainers := []Port{
		{Number: 22},
		{Name: "http", Number: 3000},
		{Name: "metrics", Number: 9090},
	}

	tests := []struct {
		target string
		want   []Port
	}{
		// The ports of the service itself, the ones the forward targets
		{"svc/grafana", []Port{{Name: "web", Number: 80}, {Name: "metrics", Number: 9090}}},
		{"deploy/grafana", fromContainers},
		{"sts/grafana", fromContainers},
		{"pod/grafana-1", fromContainers},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			target, err := ParseTarget(tt.target)
			if err != nil {
				t.Fatalf("ParseTarget: %v", err)
			}
			got, err := Ports(t.Context(), cs, ns, target)
			if err != nil {
				t.Fatalf("Ports: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPortsTargetMissing(t *testing.T) {
	for _, target := range []string{"svc/absent", "deploy/absent", "sts/absent", "pod/absent"} {
		t.Run(target, func(t *testing.T) {
			parsed, _ := ParseTarget(target)
			if _, err := Ports(t.Context(), fake.NewClientset(), ns, parsed); !apierrors.IsNotFound(
				err,
			) {
				t.Errorf("got error %v, want NotFound", err)
			}
		})
	}
}

// Every kind accepted by ParseTarget has a short name to be proposed with.
func TestShortKinds(t *testing.T) {
	for _, kind := range kindAliases {
		if shortKinds[kind] == "" {
			t.Errorf("kind %q has no short name", kind)
		}
	}
}
