package config

import (
	"os"
	"path/filepath"
)

func DataDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".otto")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ".otto"
	}
	return filepath.Join(home, ".otto")
}
