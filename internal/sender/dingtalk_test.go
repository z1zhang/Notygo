package sender

import "testing"

func TestFallbackTitle(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"first line", "line one\nline two", "line one"},
		{"skips blank lines", "\n\n  \nactual title\nbody", "actual title"},
		{"truncates long line", string(make([]byte, 0)) + repeat('a', 40), repeat('a', 32) + "..."},
		{"empty body", "", "通知"},
		{"whitespace body", "   \n  ", "通知"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fallbackTitle(tt.body); got != tt.want {
				t.Errorf("fallbackTitle(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func repeat(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
