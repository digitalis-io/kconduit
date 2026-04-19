package config

import (
	"os"
	"path/filepath"
	"testing"
)

// overrideConfigPath redirects ConfigPath to a temp file for testing.
// It patches os.UserHomeDir by overriding the path inline — we test via a helper.
func tempConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sessions.yaml")
}

// withConfigPath swaps ConfigPath to return the given path during the test.
func withConfigPath(path string, fn func()) {
	orig := configPathOverride
	configPathOverride = func() string { return path }
	defer func() { configPathOverride = orig }()
	fn()
}

func TestSaveAndLoad(t *testing.T) {
	path := tempConfigPath(t)
	withConfigPath(path, func() {
		s := Session{
			Brokers:  "localhost:9092",
			LogLevel: "debug",
			AIEngine: "ollama",
			SASL: SessionSASL{
				Enabled:   true,
				Mechanism: "PLAIN",
				Username:  "admin",
				Protocol:  "SASL_PLAINTEXT",
			},
		}
		if err := Save("local-dev", s); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := Load("local-dev")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got.Brokers != s.Brokers {
			t.Errorf("Brokers: got %q want %q", got.Brokers, s.Brokers)
		}
		if got.SASL.Username != s.SASL.Username {
			t.Errorf("SASL.Username: got %q want %q", got.SASL.Username, s.SASL.Username)
		}
	})
}

func TestLoadNotFound(t *testing.T) {
	path := tempConfigPath(t)
	withConfigPath(path, func() {
		// Save one session, try to load a different one
		_ = Save("exists", Session{Brokers: "localhost:9092"})
		_, err := Load("does-not-exist")
		if err == nil {
			t.Fatal("expected error loading non-existent session")
		}
	})
}

func TestList(t *testing.T) {
	path := tempConfigPath(t)
	withConfigPath(path, func() {
		// Empty file returns nil slice, no error
		names, err := List()
		if err != nil {
			t.Fatalf("List on missing file: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("expected empty list, got %v", names)
		}

		_ = Save("bravo", Session{Brokers: "b:9092"})
		_ = Save("alpha", Session{Brokers: "a:9092"})
		_ = Save("charlie", Session{Brokers: "c:9092"})

		names, err = List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(names) != 3 {
			t.Fatalf("expected 3 sessions, got %d", len(names))
		}
		// Must be sorted
		if names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
			t.Errorf("expected sorted [alpha bravo charlie], got %v", names)
		}
	})
}

func TestDelete(t *testing.T) {
	path := tempConfigPath(t)
	withConfigPath(path, func() {
		_ = Save("keep", Session{Brokers: "a:9092"})
		_ = Save("remove", Session{Brokers: "b:9092"})

		if err := Delete("remove"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		names, _ := List()
		if len(names) != 1 || names[0] != "keep" {
			t.Errorf("expected [keep] after delete, got %v", names)
		}

		// Delete non-existent session should error
		if err := Delete("remove"); err == nil {
			t.Fatal("expected error deleting non-existent session")
		}
	})
}

func TestSaveCreatesSecureFile(t *testing.T) {
	path := tempConfigPath(t)
	withConfigPath(path, func() {
		if err := Save("test", Session{Brokers: "localhost:9092"}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("expected file permissions 0600, got %o", perm)
		}
	})
}
