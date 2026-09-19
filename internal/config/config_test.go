package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultIsSafe(t *testing.T) {
	cfg := Default()

	if cfg.DefaultBaseBranch == "" {
		t.Error("DefaultBaseBranch = \"\", want a usable base branch")
	}
	if !cfg.ConfirmDestructive {
		t.Error("ConfirmDestructive = false, want true so destructive operations require confirmation")
	}
	if cfg.BranchNaming.Separator != "-" {
		t.Errorf("BranchNaming.Separator = %q, want %q", cfg.BranchNaming.Separator, "-")
	}
	if cfg.BranchNaming.MaxLength != 0 {
		t.Errorf("BranchNaming.MaxLength = %d, want 0 for no limit", cfg.BranchNaming.MaxLength)
	}
}

func TestLoadOverridesBranchNaming(t *testing.T) {
	path := writeConfig(t, `{"branchNaming":{"separator":"_","maxLength":50}}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.BranchNaming.Separator != "_" {
		t.Errorf("Separator = %q, want %q", cfg.BranchNaming.Separator, "_")
	}
	if cfg.BranchNaming.MaxLength != 50 {
		t.Errorf("MaxLength = %d, want 50", cfg.BranchNaming.MaxLength)
	}
}

// Naming is a nested object, so a partial one must not blank out the fields it
// does not mention.
func TestLoadKeepsNamingDefaultsForAbsentFields(t *testing.T) {
	path := writeConfig(t, `{"branchNaming":{"maxLength":40}}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.BranchNaming.Separator != "-" {
		t.Errorf("Separator = %q, want the default %q", cfg.BranchNaming.Separator, "-")
	}
	if cfg.BranchNaming.MaxLength != 40 {
		t.Errorf("MaxLength = %d, want 40", cfg.BranchNaming.MaxLength)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg != Default() {
		t.Errorf("Load() = %+v, want the defaults %+v", cfg, Default())
	}
}

func TestLoadEmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg != Default() {
		t.Errorf("Load() = %+v, want the defaults %+v", cfg, Default())
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	path := writeConfig(t, `{"defaultBaseBranch":"develop","confirmDestructive":false}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DefaultBaseBranch != "develop" {
		t.Errorf("DefaultBaseBranch = %q, want %q", cfg.DefaultBaseBranch, "develop")
	}
	if cfg.ConfirmDestructive {
		t.Error("ConfirmDestructive = true, want false")
	}
}

func TestLoadKeepsDefaultsForAbsentFields(t *testing.T) {
	path := writeConfig(t, `{"defaultBaseBranch":"develop"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.DefaultBaseBranch != "develop" {
		t.Errorf("DefaultBaseBranch = %q, want %q", cfg.DefaultBaseBranch, "develop")
	}
	if !cfg.ConfirmDestructive {
		t.Error("ConfirmDestructive = false, want the default true")
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	path := writeConfig(t, `{"defaultBaseBranch":`)

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
	if cfg != Default() {
		t.Errorf("Load() = %+v, want the defaults %+v", cfg, Default())
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("Load() error = %v, want it to name the file", err)
	}
}

func TestLoadReportsUnreadableFile(t *testing.T) {
	// A directory cannot be read as a configuration file.
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
	if cfg != Default() {
		t.Errorf("Load() = %+v, want the defaults %+v", cfg, Default())
	}
}

func TestDefaultPathPointsAtTheGConfigDirectory(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Skip("no user configuration directory is available on this platform")
	}
	if want := filepath.Join("g", FileName); !strings.HasSuffix(path, want) {
		t.Errorf("DefaultPath() = %q, want it to end with %q", path, want)
	}
}

// writeConfig writes a configuration file into a temporary directory.
func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}
