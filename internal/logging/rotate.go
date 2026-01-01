package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func rotateFiles(basePath string, maxBackups int) error {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	backups, err := findBackups(dir, name, ext)
	if err != nil {
		return err
	}

	sort.Sort(sort.Reverse(sort.IntSlice(backups)))

	for _, num := range backups {
		if num >= maxBackups {
			oldPath := filepath.Join(dir, fmt.Sprintf("%s.%d%s", name, num, ext))
			os.Remove(oldPath)
			continue
		}
		oldPath := filepath.Join(dir, fmt.Sprintf("%s.%d%s", name, num, ext))
		newPath := filepath.Join(dir, fmt.Sprintf("%s.%d%s", name, num+1, ext))
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rotate %s to %s: %w", oldPath, newPath, err)
		}
	}

	if _, err := os.Stat(basePath); err == nil {
		rotatedPath := filepath.Join(dir, fmt.Sprintf("%s.1%s", name, ext))
		if err := os.Rename(basePath, rotatedPath); err != nil {
			return fmt.Errorf("failed to rotate current log: %w", err)
		}
	}

	return nil
}

func findBackups(dir, name, ext string) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var backups []int
	prefix := name + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fname := entry.Name()
		if !strings.HasPrefix(fname, prefix) {
			continue
		}
		if !strings.HasSuffix(fname, ext) {
			continue
		}

		numStr := strings.TrimSuffix(strings.TrimPrefix(fname, prefix), ext)
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		backups = append(backups, num)
	}

	return backups, nil
}
