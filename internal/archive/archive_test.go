package archive_test

import (
	"os"
	"testing"

	"github.com/dobyte/closed-source-solution/internal/archive"
)

func TestZip(t *testing.T) {
	srcDir := "../../doudizhu"
	dstDir := "doudizhu"

	buf, err := archive.Zip(srcDir)
	if err != nil {
		t.Fatal(err)
	}

	if err = os.RemoveAll(dstDir); err != nil {
		t.Fatal(err)
	}

	if err = archive.Unzip(buf, dstDir); err != nil {
		t.Fatal(err)
	}
}
