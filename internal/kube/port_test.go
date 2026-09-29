package kube

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func testService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "grafana"},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
			{Name: "http", Port: 80, TargetPort: intstr.FromString("web")},
			{Name: "metrics", Port: 9090, TargetPort: intstr.FromInt32(9091)},
			{Name: "same", Port: 8080},
			{Name: "dns-udp", Port: 53, Protocol: corev1.ProtocolUDP},
			{Name: "dns-tcp", Port: 5353, Protocol: corev1.ProtocolTCP},
			{Name: "broken", Port: 7000, TargetPort: intstr.FromString("absent")},
		}},
	}
}

func testPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "grafana-1"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: "sidecar", Ports: []corev1.ContainerPort{{Name: "probe", ContainerPort: 8081}}},
			{Name: "grafana", Ports: []corev1.ContainerPort{{Name: "web", ContainerPort: 3000}}},
		}},
	}
}

func TestPodPort(t *testing.T) {
	tests := []struct {
		name    string
		svc     *corev1.Service
		remote  intstr.IntOrString
		want    int32
		wantErr string // empty = must succeed
	}{
		{"svc, number to named targetPort", testService(), intstr.FromInt32(80), 3000, ""},
		{"svc, name to named targetPort", testService(), intstr.FromString("http"), 3000, ""},
		{"svc, numeric targetPort", testService(), intstr.FromInt32(9090), 9091, ""},
		{"svc, omitted targetPort", testService(), intstr.FromInt32(8080), 8080, ""},
		{"svc, explicit TCP", testService(), intstr.FromString("dns-tcp"), 5353, ""},
		{"svc, UDP port ignored", testService(), intstr.FromInt32(53), 0, "no TCP service port 53"},
		{
			"svc, UDP port by name",
			testService(),
			intstr.FromString("dns-udp"),
			0,
			"no TCP service port dns-udp",
		},
		{"svc, unknown port", testService(), intstr.FromInt32(81), 0, "no TCP service port 81"},
		{
			"svc, targetPort not in pod",
			testService(),
			intstr.FromInt32(7000),
			0,
			`no container port named "absent"`,
		},
		{"pod, name in second container", nil, intstr.FromString("web"), 3000, ""},
		{"pod, name in first container", nil, intstr.FromString("probe"), 8081, ""},
		{"pod, number used as is", nil, intstr.FromInt32(1234), 1234, ""},
		{
			"pod, unknown name",
			nil,
			intstr.FromString("nope"),
			0,
			`pod grafana-1 has no container port named "nope"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := podPort(tt.remote, tt.svc, testPod())

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("podPort: %v", err)
			}
			check(t, "port", got, tt.want)
		})
	}
}
