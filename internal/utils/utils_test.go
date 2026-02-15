package utils

import (
	"testing"
)

func TestGenerateCode(t *testing.T) {
	tests := []struct {
		length int
	}{
		{4},
		{6},
		{8},
	}

	for _, tt := range tests {
		code := GenerateCode(tt.length)
		if len(code) != tt.length {
			t.Errorf("GenerateCode(%d) length = %d; want %d", tt.length, len(code), tt.length)
		}
		// check if numeric
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("GenerateCode(%d) contains non-numeric character: %c", tt.length, c)
			}
		}
	}
}
