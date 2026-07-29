package agent

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		detected []string
		want     string
	}{
		{"explicit wins", "claude", []string{"crush", "codex"}, "claude"},
		{"none detected", "", nil, None},
		{"single detected", "", []string{"crush"}, "crush"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.explicit, tt.detected); got != tt.want {
				t.Errorf("Resolve(%q, %v) = %q, want %q", tt.explicit, tt.detected, got, tt.want)
			}
		})
	}
}
