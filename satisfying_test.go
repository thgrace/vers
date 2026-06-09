package vers

import (
	"reflect"
	"testing"
)

func TestSatisfying(t *testing.T) {
	tests := []struct {
		name       string
		versions   []string
		constraint string
		scheme     string
		want       []string
	}{
		{
			name:       "npm caret preserves input order",
			versions:   []string{"2.0.0", "1.2.3", "1.9.9", "1.2.2"},
			constraint: "^1.2.3",
			scheme:     "npm",
			want:       []string{"1.2.3", "1.9.9"},
		},
		{
			name:       "npm range with exclusion",
			versions:   []string{"0.9.9", "1.0.0", "1.5.0", "1.6.0", "2.0.0"},
			constraint: ">=1.0.0 <2.0.0 !=1.5.0",
			scheme:     "npm",
			want:       []string{"1.0.0", "1.6.0"},
		},
		{
			name:       "vers URI",
			versions:   []string{"1.0.0", "1.4.0", "1.5.0", "2.0.0"},
			constraint: "vers:npm/>=1.0.0|<2.0.0|!=1.5.0",
			scheme:     "",
			want:       []string{"1.0.0", "1.4.0"},
		},
		{
			name:       "pypi compatible release",
			versions:   []string{"1.4.1", "1.4.2", "1.4.9", "1.5.0"},
			constraint: "~=1.4.2",
			scheme:     "pypi",
			want:       []string{"1.4.2", "1.4.9"},
		},
		{
			name:       "pypi comparators with exclusion",
			versions:   []string{"0.9.0", "1.0.0", "1.5.0", "1.8.0", "2.0.0"},
			constraint: ">=1.0.0,<2.0.0,!=1.5.0",
			scheme:     "pypi",
			want:       []string{"1.0.0", "1.8.0"},
		},
		{
			name:       "invalid candidates are skipped by containment behavior",
			versions:   []string{"invalid", "1.0.0", "also-invalid", "2.0.0"},
			constraint: "^1.0.0",
			scheme:     "npm",
			want:       []string{"1.0.0"},
		},
		{
			name:       "empty input",
			versions:   nil,
			constraint: "^1.0.0",
			scheme:     "npm",
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Satisfying(tt.versions, tt.constraint, tt.scheme)
			if err != nil {
				t.Fatalf("Satisfying() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Satisfying() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSatisfyingInvalidConstraint(t *testing.T) {
	_, err := Satisfying([]string{"1.0.0"}, ">", "npm")
	if err == nil {
		t.Fatal("Satisfying() error = nil, want non-nil")
	}
}
