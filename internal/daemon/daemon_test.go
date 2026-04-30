package daemon

import (
	"path/filepath"
	"testing"
)

func TestStatusRemovesStalePIDFile(t *testing.T) {
	t.Parallel()

	pidPath := filepath.Join(t.TempDir(), "retrace.pid")
	if err := writePID(pidPath, 99999999); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	pid, running, err := Status(Config{PIDPath: pidPath})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if pid != 99999999 {
		t.Fatalf("pid mismatch: got %d", pid)
	}
	if running {
		t.Fatal("expected stale pid to be reported as stopped")
	}
}
