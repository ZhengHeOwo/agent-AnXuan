package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func validateToolPath(input string) (string, error) {
	if input == "." {
		return "", ErrInvalidPath
	}

	if !fs.ValidPath(input) {
		return "", ErrInvalidPath
	}

	if strings.ContainsAny(input, "\\:\x00") {
		return "", ErrInvalidPath
	}

	return input, nil
}

func localizeToolPath(toolPath string) (string, error) {
	localPath, err := filepath.Localize(toolPath)
	if err != nil {
		return "", fmt.Errorf("localize path %v: %w", toolPath, err)
	}

	return localPath, nil
}

func (w *Workspace) rejectSymlinkPath(localPath string) error {
	current := ""

	for _, component := range strings.Split(localPath, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := w.root.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect path component %v: %w", current, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("inspect path component %v: %w", current, ErrSymlinkPath)
		}
	}

	return nil
}
