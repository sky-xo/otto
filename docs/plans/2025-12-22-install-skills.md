# Install Skills Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `june install-skills` to copy repo skills into `~/.claude/skills/`, overwriting only `june-*` skills and printing a summary.

**Architecture:** Introduce a new Cobra command that calls a small helper to copy skill directories from `skills/` under the repo root to the user skills directory. The helper returns the list of installed skills; the command prints a concise summary. Only existing destination directories with the `june-` prefix are overwritten.

**Tech Stack:** Go, Cobra, stdlib `os`, `io`, `io/fs`, `path/filepath`

### Task 1: Write the failing test for copy/overwrite behavior

**Files:**
- Create: `internal/cli/commands/install_skills_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -run TestRunInstallSkillsCopiesAndOverwritesJuneOnly`
Expected: FAIL with `runInstallSkills` undefined.

**Step 3: Commit**

```bash
git add internal/cli/commands/install_skills_test.go
git commit -m "test: cover install-skills copy semantics"
```

### Task 2: Implement the install-skills command and copy helper

**Files:**
- Create: `internal/cli/commands/install_skills.go`
- Modify: `internal/cli/root.go`

**Step 1: Write minimal implementation**

```go
func NewInstallSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-skills",
		Short: "Install bundled June skills into ~/.claude/skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceRoot := scope.RepoRoot()
			if sourceRoot == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				sourceRoot = cwd
			}
			source := filepath.Join(sourceRoot, "skills")

			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dest := filepath.Join(home, ".claude", "skills")

			installed, err := runInstallSkills(source, dest)
			if err != nil {
				return err
			}

			if len(installed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No skills installed")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed skills (%d): %s\n", len(installed), strings.Join(installed, ", "))
			return nil
		},
	}
	return cmd
}

func runInstallSkills(source, dest string) ([]string, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}

	var installed []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		destPath := filepath.Join(dest, name)
		if _, err := os.Stat(destPath); err == nil {
			if !strings.HasPrefix(name, "june-") {
				continue
			}
			if err := os.RemoveAll(destPath); err != nil {
				return nil, err
			}
		}

		sourcePath := filepath.Join(source, name)
		if err := copyDir(sourcePath, destPath); err != nil {
			return nil, err
		}
		installed = append(installed, name)
	}

	return installed, nil
}

func copyDir(source, dest string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		outPath := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(outPath, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
}
```

**Step 2: Wire into root command**

```go
rootCmd.AddCommand(commands.NewInstallSkillsCmd())
```

**Step 3: Run test to verify it passes**

Run: `go test ./internal/cli/commands -run TestRunInstallSkillsCopiesAndOverwritesJuneOnly`
Expected: PASS.

**Step 4: Commit**

```bash
git add internal/cli/commands/install_skills.go internal/cli/root.go
git commit -m "feat: add install-skills command"
```

### Task 3: Expand test coverage for summary output (optional)

**Files:**
- Modify: `internal/cli/commands/install_skills_test.go`

**Step 1: Write failing test for summary message**

```go
func TestInstallSkillsCommandPrintsSummary(t *testing.T) {
	// Set up temp source/dest, then run NewInstallSkillsCmd() with a buffer
	// and assert on "Installed skills" line.
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/commands -run TestInstallSkillsCommandPrintsSummary`
Expected: FAIL due to missing setup or output mismatch.

**Step 3: Implement minimal adjustments**

- If needed, inject a helper to build the command with an overridden source root.
- Keep production behavior unchanged.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/commands -run TestInstallSkillsCommandPrintsSummary`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/commands/install_skills_test.go
git commit -m "test: cover install-skills summary output"
```

### Notes

- Use @superpowers:test-driven-development and keep each step small.
- Prefer returning a sorted `installed` list if deterministic output is needed.
- Skip non-directory entries under `skills/`.
