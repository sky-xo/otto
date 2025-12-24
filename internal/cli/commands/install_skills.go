package commands

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"otto/internal/scope"

	"github.com/spf13/cobra"
)

func NewInstallSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-skills",
		Short: "Install bundled Otto skills into ~/.claude/skills and ~/.codex/skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceRoot := scope.RepoRoot()
			if sourceRoot == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				sourceRoot = cwd
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			var allInstalled []string

			// Install Claude skills if ~/.claude exists
			claudeHome := filepath.Join(home, ".claude")
			if _, err := os.Stat(claudeHome); err == nil {
				claudeSource := filepath.Join(sourceRoot, ".claude", "skills")
				claudeDest := filepath.Join(claudeHome, "skills")
				installed, err := runInstallSkills(claudeSource, claudeDest)
				if err != nil {
					return fmt.Errorf("installing claude skills: %w", err)
				}
				for _, s := range installed {
					allInstalled = append(allInstalled, s+" (claude)")
				}
			}

			// Install Codex skills if ~/.codex exists
			codexHome := filepath.Join(home, ".codex")
			if _, err := os.Stat(codexHome); err == nil {
				codexSource := filepath.Join(sourceRoot, ".codex", "skills")
				codexDest := filepath.Join(codexHome, "skills")
				installed, err := runInstallSkills(codexSource, codexDest)
				if err != nil {
					return fmt.Errorf("installing codex skills: %w", err)
				}
				for _, s := range installed {
					allInstalled = append(allInstalled, s+" (codex)")
				}
			}

			if len(allInstalled) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No skills installed (neither ~/.claude nor ~/.codex found)")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed skills (%d): %s\n", len(allInstalled), strings.Join(allInstalled, ", "))
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
			if !strings.HasPrefix(name, "otto-") {
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
