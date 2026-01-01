package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunInstallSkillsCopiesAndOverwritesJuneOnly(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	source := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}

	juneSkill := filepath.Join(source, "june-orchestrate")
	userSkill := filepath.Join(source, "user-skill")
	if err := os.MkdirAll(juneSkill, 0o755); err != nil {
		t.Fatalf("mkdir june: %v", err)
	}
	if err := os.MkdirAll(userSkill, 0o755); err != nil {
		t.Fatalf("mkdir user: %v", err)
	}
	if err := os.WriteFile(filepath.Join(juneSkill, "SKILL.md"), []byte("june new"), 0o644); err != nil {
		t.Fatalf("write june: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkill, "SKILL.md"), []byte("user new"), 0o644); err != nil {
		t.Fatalf("write user: %v", err)
	}

	dest := filepath.Join(tempHome, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(dest, "june-orchestrate"), 0o755); err != nil {
		t.Fatalf("mkdir dest june: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "user-skill"), 0o755); err != nil {
		t.Fatalf("mkdir dest user: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "june-orchestrate", "SKILL.md"), []byte("june old"), 0o644); err != nil {
		t.Fatalf("write dest june: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "user-skill", "SKILL.md"), []byte("user old"), 0o644); err != nil {
		t.Fatalf("write dest user: %v", err)
	}

	installed, err := runInstallSkills(source, dest)
	if err != nil {
		t.Fatalf("runInstallSkills: %v", err)
	}

	if len(installed) != 1 || installed[0] != "june-orchestrate" {
		t.Fatalf("expected only june-orchestrate installed, got %v", installed)
	}

	juneBytes, _ := os.ReadFile(filepath.Join(dest, "june-orchestrate", "SKILL.md"))
	userBytes, _ := os.ReadFile(filepath.Join(dest, "user-skill", "SKILL.md"))
	if string(juneBytes) != "june new" {
		t.Fatalf("expected june skill overwritten, got %q", string(juneBytes))
	}
	if string(userBytes) != "user old" {
		t.Fatalf("expected user skill preserved, got %q", string(userBytes))
	}
}

func TestRunInstallSkillsCreatesDestDir(t *testing.T) {
	source := t.TempDir()
	juneSkill := filepath.Join(source, "june-test")
	if err := os.MkdirAll(juneSkill, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(juneSkill, "SKILL.md"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "nonexistent", "skills")
	installed, err := runInstallSkills(source, dest)
	if err != nil {
		t.Fatalf("runInstallSkills: %v", err)
	}

	if len(installed) != 1 || installed[0] != "june-test" {
		t.Fatalf("expected june-test installed, got %v", installed)
	}

	content, err := os.ReadFile(filepath.Join(dest, "june-test", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed skill: %v", err)
	}
	if string(content) != "content" {
		t.Fatalf("expected 'content', got %q", string(content))
	}
}
