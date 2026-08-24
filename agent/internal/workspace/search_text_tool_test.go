package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchTextTool_returns_structured_matches(
	t *testing.T,
) {
	dir := t.TempDir()

	localPath := filepath.Join(
		dir,
		"internal",
		"workspace.go",
	)

	if err := os.MkdirAll(
		filepath.Dir(localPath),
		0o755,
	); err != nil {
		t.Fatalf(
			"os.MkdirAll(%q) error = %v, want nil",
			filepath.Dir(localPath),
			err,
		)
	}

	content := "" +
		"package internal\n" +
		"func OpenWorkspace() {}\n"

	if err := os.WriteFile(
		localPath,
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatalf(
			"os.WriteFile(%q) error = %v, want nil",
			localPath,
			err,
		)
	}

	projectWorkspace, err := OpenWorkspace(dir)
	if err != nil {
		t.Fatalf(
			"OpenWorkspace(%q) error = %v, want nil",
			dir,
			err,
		)
	}
	t.Cleanup(func() {
		_ = projectWorkspace.Close()
	})

	searchTool, err := NewSearchTextTool(
		projectWorkspace,
	)
	if err != nil {
		t.Fatalf(
			"NewSearchTextTool() error = %v, want nil",
			err,
		)
	}

	encoded, err := searchTool.Execute(
		context.Background(),
		json.RawMessage(
			`{"query":"OpenWorkspace"}`,
		),
	)
	if err != nil {
		t.Fatalf(
			"Execute() error = %v, want nil",
			err,
		)
	}

	var response searchTextResponse
	if err := json.Unmarshal(
		[]byte(encoded),
		&response,
	); err != nil {
		t.Fatalf(
			"json.Unmarshal() error = %v, want nil",
			err,
		)
	}

	if len(response.Matches) != 1 {
		t.Fatalf(
			"len(response.Matches) = %d, want 1",
			len(response.Matches),
		)
	}

	match := response.Matches[0]

	if match.Path != "internal/workspace.go" {
		t.Fatalf(
			"match.Path = %q, want %q",
			match.Path,
			"internal/workspace.go",
		)
	}

	if match.Line != 2 {
		t.Fatalf(
			"match.Line = %d, want 2",
			match.Line,
		)
	}

	if match.Text != "func OpenWorkspace() {}" {
		t.Fatalf(
			"match.Text = %q, want %q",
			match.Text,
			"func OpenWorkspace() {}",
		)
	}

	if match.TextTruncated {
		t.Fatal(
			"match.TextTruncated = true, want false",
		)
	}

	if response.Truncated {
		t.Fatal(
			"response.Truncated = true, want false",
		)
	}
}
