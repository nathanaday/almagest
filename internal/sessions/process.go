package sessions

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// harnessNames are the process names of the agents whose sessions Atlas records.
var harnessNames = []string{"claude", "codex"}

// MaxAncestors bounds the walk up the process tree from a hook to its agent.
const MaxAncestors = 8

// processInfo reads a process's parent and command name with ps.
func processInfo(pid int) (ppid int, name string, ok bool) {
	out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "", false
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return 0, "", false
	}
	ppid, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, "", false
	}
	return ppid, filepath.Base(strings.Join(fields[1:], " ")), true
}

func isHarness(name string) bool {
	for _, h := range harnessNames {
		if name == h || strings.HasPrefix(name, h+"-") {
			return true
		}
	}
	return false
}

// HarnessPID is the id of the agent process that runs this hook: the nearest ancestor
// whose name is claude or codex, or 0. A hook runs as a child of the agent, through a
// shell and the plugin's wrapper script.
func HarnessPID() int {
	pid := os.Getppid()
	for i := 0; i < MaxAncestors && pid > 1; i++ {
		ppid, name, ok := processInfo(pid)
		if !ok {
			return 0
		}
		if isHarness(name) {
			return pid
		}
		pid = ppid
	}
	return 0
}

// Alive reports whether the agent process of a session still runs: the process exists,
// and its name is an agent's, so a reused id of another program does not count.
func Alive(pid int) bool {
	if pid <= 1 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	_, name, ok := processInfo(pid)
	return ok && isHarness(name)
}
