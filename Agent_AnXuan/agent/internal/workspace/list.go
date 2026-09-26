package workspace

import (
	"context"
	"fmt"
	"io/fs"
)

const (
	maxListEntries = 2_000
	maxListedFiles = 1500
)

type textFileList struct {
	paths     []string
	truncated bool
}

func (w *Workspace) ListTextFiles(
	ctx context.Context,
) (*textFileList, error) {
	result := &textFileList{
		paths: make([]string, 0),
	}

	scannedEntries := 0

	err := fs.WalkDir(
		w.root.FS(),
		".",
		func(toolPath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf(
					"walk path %v: %w",
					toolPath,
					walkErr,
				)
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if toolPath == "." {
				return nil
			}

			scannedEntries++
			if scannedEntries > maxListEntries {
				result.truncated = true
				return fs.SkipAll
			}

			if entry.Type()&fs.ModeSymlink != 0 {
				return nil
			}

			if entry.IsDir() {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf(
					"inspect path %v: %w",
					toolPath,
					err,
				)
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			if !isAllowedTextFile(toolPath) {
				return nil
			}

			if len(result.paths) >= maxListedFiles {
				result.truncated = true
				return fs.SkipAll
			}

			result.paths = append(result.paths, toolPath)
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf(
			"scan workspace: %w",
			err,
		)
	}

	return result, nil
}
