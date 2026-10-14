package httpapi

import "testing"

func TestJoinURLRelative(t *testing.T) {
	if got := joinURL("https://mcpic.example", "/i/abc"); got != "https://mcpic.example/i/abc" {
		t.Fatalf("joinURL relative = %q", got)
	}
}

func TestJoinURLAbsoluteHTTPS(t *testing.T) {
	if got := joinURL("https://mcpic.example", "https://cdn.example/abc"); got != "https://cdn.example/abc" {
		t.Fatalf("joinURL absolute = %q", got)
	}
}

func TestJoinURLAbsoluteHTTP(t *testing.T) {
	if got := joinURL("https://mcpic.example", "http://cdn.example/abc"); got != "http://cdn.example/abc" {
		t.Fatalf("joinURL absolute http = %q", got)
	}
}
