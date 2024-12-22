package schema

import "testing"

func TestCircularError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "basic error message",
			message: "circular dependency detected",
			want:    "circular dependency detected",
		},
		{
			name:    "empty message",
			message: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &CircularError{msg: tt.message}
			if got := err.Error(); got != tt.want {
				t.Errorf("CircularError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
