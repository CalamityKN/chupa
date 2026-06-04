package main

import "testing"

func TestNormalizeManifestRelAcceptsSafePaths(t *testing.T) {
	got, err := normalizeManifestRel(`nested folder\b file.txt`)
	if err != nil {
		t.Fatalf("normalizeManifestRel returned error: %v", err)
	}
	if got != "nested folder/b file.txt" {
		t.Fatalf("got %q, want %q", got, "nested folder/b file.txt")
	}
}

func TestNormalizeManifestRelRejectsUnsafePaths(t *testing.T) {
	tests := []string{
		"../escape.txt",
		"nested/../../escape.txt",
		"/absolute/path.txt",
		`C:\absolute\path.txt`,
		"",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := normalizeManifestRel(input); err == nil {
				t.Fatalf("expected %q to be rejected", input)
			}
		})
	}
}
