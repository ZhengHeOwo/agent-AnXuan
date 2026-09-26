package workspace

import (
	"path"
)

func isAllowedTextFile(toolPath string) bool {
	filename := path.Base(toolPath)
	switch filename {
	case "go.mod", "go.sum":
		return true
	}

	extension := path.Ext(filename)
	switch extension {
	case ".go", ".md", ".txt", ".json", ".yaml", ".yml", ".toml":
		return true
	}

	return false
}
