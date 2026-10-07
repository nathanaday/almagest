package hooks_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/hooks"
)

// A hook whose lock another write holds gives up within its deadline, with an error that
// names the lock, whether the holder is in this process or another.
func TestAHookGivesUpOnAHeldLockWithinItsDeadline(t *testing.T) {
	f := setup(t)
	f.run("session-start", map[string]any{})
	old := hooks.Deadlines["prompt"]
	hooks.Deadlines["prompt"] = 300 * time.Millisecond
	defer func() { hooks.Deadlines["prompt"] = old }()
	prompt := func() (time.Duration, error) {
		data, _ := json.Marshal(map[string]any{"session_id": sid, "cwd": f.tv.V.Root, "prompt": "yes"})
		began := time.Now()
		err := hooks.Run("prompt", bytes.NewReader(data), &bytes.Buffer{}, f.env())
		return time.Since(began), err
	}

	unlock, err := f.tv.V.Lock()
	if err != nil {
		t.Fatal(err)
	}
	took, err := prompt()
	unlock()
	if err == nil || !strings.Contains(err.Error(), "lock") || took > 2*time.Second {
		t.Fatalf("with the lock held in this process: took %s, err %v", took, err)
	}

	// Another process holds the file lock: a second open file takes flock as another
	// holder would.
	lock, err := os.OpenFile(filepath.Join(f.tv.V.Root, ".git", "almagest.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	took, err = prompt()
	syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if err == nil || !strings.Contains(err.Error(), "almagest.lock") || took > 2*time.Second {
		t.Fatalf("with the file lock held: took %s, err %v", took, err)
	}

	if _, err := prompt(); err != nil {
		t.Fatalf("with the lock free: %v", err)
	}
}
