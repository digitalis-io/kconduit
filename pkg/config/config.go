package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// SessionSASL holds SASL settings for a saved session.
// Passwords are never stored; use PasswordFile to reference a file containing the password.
type SessionSASL struct {
	Enabled      bool   `yaml:"enabled"`
	Mechanism    string `yaml:"mechanism,omitempty"`
	Username     string `yaml:"username,omitempty"`
	Protocol     string `yaml:"protocol,omitempty"`
	PasswordFile string `yaml:"password_file,omitempty"`
}

// SessionTLS holds TLS settings for a saved session.
type SessionTLS struct {
	Enabled    bool   `yaml:"enabled"`
	CACert     string `yaml:"ca_cert,omitempty"`
	ClientCert string `yaml:"client_cert,omitempty"`
	ClientKey  string `yaml:"client_key,omitempty"`
	SkipVerify bool   `yaml:"skip_verify,omitempty"`
}

// Session is a named connection profile.
type Session struct {
	Brokers  string      `yaml:"brokers"`
	LogLevel string      `yaml:"log_level,omitempty"`
	AIEngine string      `yaml:"ai_engine,omitempty"`
	AIModel  string      `yaml:"ai_model,omitempty"`
	SASL     SessionSASL `yaml:"sasl,omitempty"`
	TLS      SessionTLS  `yaml:"tls,omitempty"`
}

type sessionFile struct {
	Sessions map[string]Session `yaml:"sessions"`
}

// configPathOverride is a hook for tests to redirect the config file location.
var configPathOverride func() string

// ConfigPath returns the path to the sessions config file (~/.config/kconduit/sessions.yaml).
func ConfigPath() string {
	if configPathOverride != nil {
		return configPathOverride()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "kconduit", "sessions.yaml")
}

// Load returns the named session from the config file, or an error if not found.
func Load(name string) (*Session, error) {
	sf, err := readFile()
	if err != nil {
		return nil, err
	}
	s, ok := sf.Sessions[name]
	if !ok {
		return nil, fmt.Errorf("session %q not found", name)
	}
	return &s, nil
}

// Save writes a session to the config file under the given name.
// The Session must not contain a plaintext password; use PasswordFile instead.
func Save(name string, s Session) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	sf, err := readFile()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read existing config: %w", err)
	}
	if sf == nil {
		sf = &sessionFile{Sessions: make(map[string]Session)}
	}

	sf.Sessions[name] = s

	data, err := yaml.Marshal(sf)
	if err != nil {
		return fmt.Errorf("failed to marshal session config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// List returns the names of all saved sessions in alphabetical order.
func List() ([]string, error) {
	sf, err := readFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(sf.Sessions))
	for name := range sf.Sessions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Delete removes the named session from the config file.
func Delete(name string) error {
	sf, err := readFile()
	if err != nil {
		return err
	}
	if _, ok := sf.Sessions[name]; !ok {
		return fmt.Errorf("session %q not found", name)
	}
	delete(sf.Sessions, name)

	path := ConfigPath()
	data, err := yaml.Marshal(sf)
	if err != nil {
		return fmt.Errorf("failed to marshal session config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// readFile reads and parses the sessions config file.
// Returns os.ErrNotExist wrapped if the file does not exist yet.
func readFile() (*sessionFile, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sf sessionFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	if sf.Sessions == nil {
		sf.Sessions = make(map[string]Session)
	}
	return &sf, nil
}
