package workspace

import (
	"fmt"
	"os"
	"strings"
)

const DefaultDir = `./AIWorkspace`

type Workspace struct {
	root *os.Root
}

func OpenWorkspace(dir string) (*Workspace, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, ErrOpenWorkspace
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpenWorkspace, err)
	}

	return &Workspace{
		root: root,
	}, nil
}

func (w *Workspace) Close() error {
	if w == nil {
		return nil
	}

	if w.root == nil {
		return nil
	}

	return w.root.Close()
}
