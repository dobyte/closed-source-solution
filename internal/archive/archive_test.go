package archive_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dobyte/closed-source-solution/internal/archive"
)

func TestZip(t *testing.T) {
	srcDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte("module test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	buf, err := archive.Zip(srcDir)
	if err != nil {
		t.Fatal(err)
	}

	dstDir := t.TempDir()

	if err = archive.Unzip(buf, dstDir); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"go.mod", "main.go"} {
		if _, err = os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Fatal(err)
		}
	}
}
