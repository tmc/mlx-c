package apilock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckIncludeGuards(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("ok.h", `#ifndef OK_H
#define OK_H
int f(void);
#endif
`)
	write("bad.h", `#ifndef BAD_H
#define BAD_H
int f(void);
#endif

inline int g(void) { return 1; }
`)
	write("private/ok2.h", `#ifndef OK2_H
#define OK2_H
#endif
/* trailing comment only */
`)
	violations, err := CheckIncludeGuards(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || !filepath.IsAbs(violations[0]) && violations[0] == "" {
		t.Fatalf("violations = %v, want exactly bad.h", violations)
	}
	if !contains(violations[0], "bad.h") {
		t.Errorf("violation = %q, want bad.h", violations[0])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
