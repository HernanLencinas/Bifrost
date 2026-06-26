package main

import (
	"os"
	"path/filepath"
	"strings"
)

// appRootDir es el directorio base para config/: el del ejecutable
// (resolviendo symlinks), salvo en builds temporales de `go run` donde se usa cwd.
func appRootDir() string {
	dir, err := executableBaseDir()
	if err != nil {
		return mustGetwd()
	}
	if isEphemeralGoBuildDir(dir) {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
	}
	return dir
}

func executableBaseDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func isEphemeralGoBuildDir(dir string) bool {
	d := filepath.Clean(dir)
	tmp := filepath.Clean(os.TempDir())
	if strings.HasPrefix(d, tmp) {
		return true
	}
	return strings.Contains(d, "go-build")
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func clientConfigPath() string {
	return filepath.Join(appRootDir(), "config", "client.conf")
}

func serverConfigPath() string {
	return filepath.Join(appRootDir(), "config", "server.conf")
}

func configDirPath() string {
	return filepath.Join(appRootDir(), "config")
}
