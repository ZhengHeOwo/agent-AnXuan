package workspace

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSearchTextFile_returns_matching_lines_in_order(
	t *testing.T,
) {
	dir := t.TempDir()

	content := "" +
		"package workspace\n" +
		"func OpenWorkspace() {}\n" +
		"func openWorkspaceForTest() {}\n" +
		"var opener = OpenWorkspace\n"

	localPath := filepath.Join(dir, "workspace.go")
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

	got, truncated, err := projectWorkspace.searchTextFile(
		context.Background(),
		"workspace.go",
		"OpenWorkspace",
	)
	if err != nil {
		t.Fatalf(
			"searchTextFile() error = %v, want nil",
			err,
		)
	}

	want := []TextMatch{
		{
			Path: "workspace.go",
			Line: 2,
			Text: "func OpenWorkspace() {}",
		},
		{
			Path: "workspace.go",
			Line: 4,
			Text: "var opener = OpenWorkspace",
		},
	}

	if !slices.Equal(got, want) {
		t.Fatalf(
			"searchTextFile() matches = %+v, want %+v",
			got,
			want,
		)
	}

	if truncated {
		t.Fatal(
			"searchTextFile() truncated = true, want false",
		)
	}
}
