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

func TestSearchText_returns_matches_in_file_and_line_order(
	t *testing.T,
) {
	dir := t.TempDir()

	files := map[string]string{
		"b.go": "" +
			"package b\n" +
			"var second = Target\n",
		"a.go": "" +
			"package a\n" +
			"var first = Target\n" +
			"var another = Target\n",
		"ignored.png": "Target",
	}

	for toolPath, content := range files {
		localPath := filepath.Join(
			dir,
			filepath.FromSlash(toolPath),
		)

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

	got, err := projectWorkspace.SearchText(
		context.Background(),
		"Target",
	)
	if err != nil {
		t.Fatalf(
			"SearchText() error = %v, want nil",
			err,
		)
	}

	want := []TextMatch{
		{
			Path: "a.go",
			Line: 2,
			Text: "var first = Target",
		},
		{
			Path: "a.go",
			Line: 3,
			Text: "var another = Target",
		},
		{
			Path: "b.go",
			Line: 2,
			Text: "var second = Target",
		},
	}

	if !slices.Equal(got.Matches, want) {
		t.Fatalf(
			"SearchText().Matches = %+v, want %+v",
			got.Matches,
			want,
		)
	}

	if got.Truncated {
		t.Fatal(
			"SearchText().Truncated = true, want false",
		)
	}
}
