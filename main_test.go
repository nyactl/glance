package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nyactl/glance/internal/reader"
)

func TestInputFromAFileOrStdin(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notes.md")
	os.WriteFile(p, []byte("from the file"), 0o600)
	if got, err := input([]string{p}); err != nil || got != "from the file" {
		t.Fatalf("file: %q %v", got, err)
	}
	if _, err := input([]string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("a missing file read")
	}
	if _, err := input([]string{"a", "b"}); err == nil {
		t.Fatal("two files read")
	}

	r, w, _ := os.Pipe()
	w.WriteString("piped in")
	w.Close()
	stdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = stdin }()
	if got, err := input(nil); err != nil || got != "piped in" {
		t.Fatalf("stdin: %q %v", got, err)
	}
}

func TestSettingsAreKept(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if s := loadSettings(); s != (reader.Settings{}) {
		t.Fatalf("first run: %+v", s)
	}
	saveSettings(reader.Settings{WPM: 425, Big: true})
	if s := loadSettings(); s != (reader.Settings{WPM: 425, Big: true}) {
		t.Fatalf("kept: %+v", s)
	}
}
