package kube

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ErrPortNotFound means the configured remote port matches no port of the
// service or of the pod: a configuration error that retrying cannot fix.
var ErrPortNotFound = errors.New("port not found")

// podPort translates the configured remote port into a container port of pod.
// For a service, remote designates a service port, by number or by name.
func podPort(remote intstr.IntOrString, svc *corev1.Service, pod *corev1.Pod) (int32, error) {
	if svc == nil {
		if remote.Type == intstr.String {
			return containerPort(pod, remote.StrVal)
		}
		return remote.IntVal, nil
	}

	sp, err := servicePort(svc, remote)
	if err != nil {
		return 0, err
	}
	switch {
	case sp.TargetPort.Type == intstr.String:
		return containerPort(pod, sp.TargetPort.StrVal)
	case sp.TargetPort.IntVal == 0:
		// targetPort missing: Kubernetes uses the same port number as the service port number
		return sp.Port, nil
	default:
		return sp.TargetPort.IntVal, nil
	}
}

func servicePort(svc *corev1.Service, remote intstr.IntOrString) (corev1.ServicePort, error) {
	for _, sp := range svc.Spec.Ports {
		// port-forward only uses TCP ; same number UDP port is ignored
		if sp.Protocol != "" && sp.Protocol != corev1.ProtocolTCP {
			continue
		}
		if (remote.Type == intstr.Int && sp.Port == remote.IntVal) ||
			(remote.Type == intstr.String && sp.Name == remote.StrVal) {
			return sp, nil
		}
	}
	return corev1.ServicePort{}, fmt.Errorf(
		"no TCP service port %s: %w",
		remote.String(),
		ErrPortNotFound,
	)
}

func containerPort(pod *corev1.Pod, name string) (int32, error) {
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == name {
				return p.ContainerPort, nil
			}
		}
	}
	return 0, fmt.Errorf(
		"pod %s has no container port named %q: %w",
		pod.Name,
		name,
		ErrPortNotFound,
	)
}
