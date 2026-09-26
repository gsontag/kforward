package kube

import (
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input string
		want  Target
	}{
		{"svc/grafana", Target{KindService, "grafana"}},
		{"service/grafana", Target{KindService, "grafana"}},
		{"services/grafana", Target{KindService, "grafana"}},
		{"SVC/grafana", Target{KindService, "grafana"}},
		{"sts/mariadb", Target{KindStatefulSet, "mariadb"}},
		{"deploy/web", Target{KindDeployment, "web"}},
		{"po/grafana-abc123", Target{KindPod, "grafana-abc123"}},
		// Like kubectl: a bare name designates a pod
		{"grafana-abc123", Target{KindPod, "grafana-abc123"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if err != nil {
				t.Fatalf("ParseTarget(%q): %v", tt.input, err)
			}
			check(t, "Target", got, tt.want)
		})
	}
}

func TestParseTargetErrors(t *testing.T) {
	tests := []struct {
		input   string
		wantErr string
	}{
		{"cronjob/backup", `unsupported kind "cronjob"`},
		{"svc/", "expected KIND/NAME"},
		{"svc/a/b", "expected KIND/NAME"},
		{"", "expected KIND/NAME"},
		{"/grafana", `unsupported kind ""`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
			check(t, "Target", got, Target{})
		})
	}
}

func TestTargetString(t *testing.T) {
	target, err := ParseTarget("svc/grafana")
	if err != nil {
		t.Fatalf("ParseTarget: %v", err)
	}
	// Canonical form, whatever the alias used
	check(t, "String", target.String(), "service/grafana")
}
