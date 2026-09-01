// Package fileio updates files the CLI does not own without losing what was
// there before: a write lands atomically, so an interrupted run cannot leave a
// half written config behind, and the first write keeps a copy of the original
// next to it.
package fileio

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// BackupSuffix marks the copy taken of a file before the CLI first writes it.
const BackupSuffix = ".requesty.bak"

// BackupAndWrite copies path once, then replaces it with data.
func BackupAndWrite(path string, data []byte, perm fs.FileMode) error {
	if err := Backup(path); err != nil {
		return fmt.Errorf("failed to backup file: %w", err)
	}

	if err := Write(path, data, perm); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Write replaces path with data, creating any missing parent directories. The
// write goes to a temporary file in the same directory and is renamed over the
// original, so a reader sees either the old file or the new one.
func Write(path string, data []byte, perm fs.FileMode) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tmp.Close()
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("failed to chmod temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// Backup copies path next to itself, keeping its mode. It does nothing when
// path is missing, or when a backup was already taken, so the copy always holds
// the file as it was before the CLI first touched it.
func Backup(path string) error {
	backupPath := path + BackupSuffix

	exists, err := Exists(backupPath)
	if err != nil {
		return fmt.Errorf("failed to check backup exists: %w", err)
	}
	if exists {
		return nil
	}

	srcExists, err := Exists(path)
	if err != nil {
		return fmt.Errorf("failed to check source exists: %w", err)
	}
	if !srcExists {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if err := os.WriteFile(backupPath, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// Exists reports whether path is there. A missing file is not an error.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

// Mode returns the permissions of path, falling back to fallback when path does
// not exist yet. It lets an update keep the mode a file already has instead of
// imposing one.
func Mode(path string, fallback fs.FileMode) (fs.FileMode, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fallback, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	return info.Mode().Perm(), nil
}
