package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nyactl/glance/internal/reader"
)

// maxPositions bounds how many texts glance remembers where it left.
const maxPositions = 200

func stateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "glance")
}

func readJSON(name string, v any) {
	if b, err := os.ReadFile(filepath.Join(stateDir(), name)); err == nil {
		json.Unmarshal(b, v)
	}
}

// writeJSON replaces the file whole, so a crash never leaves half of it.
func writeJSON(name string, v any) {
	b, err := json.Marshal(v)
	if err != nil || os.MkdirAll(stateDir(), 0o700) != nil {
		return
	}
	p := filepath.Join(stateDir(), name)
	if os.WriteFile(p+".new", b, 0o600) == nil {
		os.Rename(p+".new", p)
	}
}

func loadSettings() reader.Settings {
	var s reader.Settings
	readJSON("settings.json", &s)
	return s
}

func saveSettings(s reader.Settings) { writeJSON("settings.json", s) }

type position struct {
	Word int   `json:"word"`
	Seen int64 `json:"seen"`
}

// textKey names a text by its content, so the same text piped in again resumes too, and a file
// that changed starts over.
func textKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:12])
}

func loadPosition(key string) int {
	ps := map[string]position{}
	readJSON("positions.json", &ps)
	return ps[key].Word
}

// savePosition keeps where a text was left; a text read to its end, or barely begun, is forgotten.
func savePosition(key string, word int, done bool) {
	ps := map[string]position{}
	readJSON("positions.json", &ps)
	if done || word == 0 {
		if _, ok := ps[key]; !ok {
			return
		}
		delete(ps, key)
	} else {
		ps[key] = position{Word: word, Seen: time.Now().Unix()}
	}
	if len(ps) > maxPositions {
		keys := make([]string, 0, len(ps))
		for k := range ps {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return ps[keys[i]].Seen > ps[keys[j]].Seen })
		for _, k := range keys[maxPositions:] {
			delete(ps, k)
		}
	}
	writeJSON("positions.json", ps)
}
