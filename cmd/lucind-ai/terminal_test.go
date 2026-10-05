package main

import (
	"os"
	"testing"
)

func TestIsTerminal_RejectsNonTerminals(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() { _ = devNull.Close() }()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = pr.Close(); _ = pw.Close() }()

	regular, err := os.Create(t.TempDir() + "/regular")
	if err != nil {
		t.Fatalf("create regular file: %v", err)
	}
	defer func() { _ = regular.Close() }()

	tests := []struct {
		name string
		file *os.File
	}{
		// /dev/null is a character device but not a terminal: this is what an agent or `!` shell gives as stdin.
		{"dev null", devNull},
		{"pipe", pr},
		{"regular file", regular},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isTerminal(tt.file) {
				t.Errorf("isTerminal(%s) = true, want false", tt.name)
			}
		})
	}
}
