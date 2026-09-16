// Package config loads the optional user configuration for g.
//
// The configuration file is deliberately small and entirely optional: a missing
// file means the built-in defaults are used, and only fields present in the file
// override them. No configuration framework is involved.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName is the configuration file name inside the g configuration directory.
const FileName = "config.json"

// Config is the user-editable configuration for g.
type Config struct {
	// DefaultBaseBranch is the branch new work branches are created from.
	DefaultBaseBranch string `json:"defaultBaseBranch"`
	// ConfirmDestructive requires explicit confirmation before an operation
	// discards commits or uncommitted work.
	ConfirmDestructive bool `json:"confirmDestructive"`
	// BranchNaming controls how branch names are built.
	BranchNaming BranchNaming `json:"branchNaming"`
}

// BranchNaming controls how a branch name is built from an issue key and a
// description.
type BranchNaming struct {
	// Separator joins the issue key and the description slug. One of "-", "_"
	// or "/"; anything else is rejected when a branch is created.
	Separator string `json:"separator"`
	// MaxLength caps the length of a generated branch name. Zero means no
	// limit. Truncation drops whole trailing words where it can.
	MaxLength int `json:"maxLength"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		DefaultBaseBranch:  "main",
		ConfirmDestructive: true,
		BranchNaming:       BranchNaming{Separator: "-"},
	}
}

// DefaultPath returns the location of the user configuration file, or "" when
// no user configuration directory can be determined. Load treats "" as "no
// configuration".
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "g", FileName)
}

// Load reads the configuration file at path.
//
// A missing file yields the defaults. A file that cannot be read or parsed is
// reported as an error together with the defaults, so callers can decide
// whether to continue.
func Load(path string) (Config, error) {
	if path == "" {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
