package parser

import "testing"

func TestSanitizeKeepsWrappedSentenceLeadIn(t *testing.T) {
	lines := []string{
		"To control the scatter location on an additional axis, add another index",
		"array to ``indices`` and another axis to ``axes``:",
		"",
		docCodeOmittedSentinel + "cpp",
		"\x00",
	}
	got := sanitizeDocLines(lines)
	t.Logf("GOT=%q", got)
	want := "To control the scatter location on an additional axis, add another index\narray to ``indices`` and another axis to ``axes``:\n\nCode example omitted: written in C++."
	if got != want {
		t.Errorf("mismatch")
	}
}
