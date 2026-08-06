package harnesses

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func writeFile(path string, data []byte, perm fs.FileMode) (err error) {
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

func backupFile(path string) error {
	backupPath := path + ".requesty.bak"

	exists, err := fileExists(backupPath)
	if err != nil {
		return fmt.Errorf("failed to check backup exists: %w", err)
	}
	if exists {
		return nil
	}

	srcExists, err := fileExists(path)
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

func fileExists(path string) (bool, error) {
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
