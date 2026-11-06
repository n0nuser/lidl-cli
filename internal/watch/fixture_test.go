package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "api", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
