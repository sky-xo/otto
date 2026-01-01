package config

import (
	"os"
	"path/filepath"
)

func DataDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".june")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ".june"
	}
	return filepath.Join(home, ".june")
}
