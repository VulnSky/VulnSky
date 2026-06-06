package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lab.qcow2")
	if err := os.WriteFile(path, []byte("vulnsky"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, size, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if size != 7 {
		t.Fatalf("size = %d, want 7", size)
	}
	if sum != "56274f1220a65106b1b9665a93d60ed17442c6b84d5e378c6c37510191d1c80d" {
		t.Fatalf("sha256 = %s", sum)
	}
}
