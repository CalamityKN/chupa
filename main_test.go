package main

import "testing"

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "unquoted",
			input: `get C:\Windows\win.ini loot.txt`,
			want:  []string{"get", `C:\Windows\win.ini`, "loot.txt"},
		},
		{
			name:  "quoted paths",
			input: `put "local file.txt" "C:\Users\Public\Test Folder\remote file.txt"`,
			want:  []string{"put", "local file.txt", `C:\Users\Public\Test Folder\remote file.txt`},
		},
		{
			name:  "empty quoted value",
			input: `cmd "" value`,
			want:  []string{"cmd", "", "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommandLine(tt.input)
			if err != nil {
				t.Fatalf("ParseCommandLine returned error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d tokens, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("token %d got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseCommandLineUnmatchedQuote(t *testing.T) {
	if _, err := ParseCommandLine(`hash "C:\bad path`); err == nil {
		t.Fatal("expected unmatched quote error")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		value int64
		want  string
	}{
		{92, "92 B"},
		{1536, "1.50 KB"},
		{2359296, "2.25 MB"},
		{3328599654, "3.10 GB"},
	}

	for _, tt := range tests {
		if got := FormatBytes(tt.value); got != tt.want {
			t.Fatalf("FormatBytes(%d) got %q, want %q", tt.value, got, tt.want)
		}
	}
}
