package memorybudget

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

func TestConfigureKeepsLowerExplicitLimit(t *testing.T) {
	previous := debug.SetMemoryLimit(64 << 20)
	defer debug.SetMemoryLimit(previous)
	if got := Configure(); got > 64<<20 {
		t.Fatalf("raised memory limit to %d", got)
	}
}

func TestConfigureLeavesCgroupHeadroom(t *testing.T) {
	data, err := os.ReadFile("/sys/fs/cgroup/memory.max")
	if err != nil {
		t.Skip("no cgroup v2")
	}
	limit, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Skip("unlimited cgroup")
	}
	previous := debug.SetMemoryLimit(1 << 62)
	defer debug.SetMemoryLimit(previous)
	got := Configure()
	if got != limit/10*7 || debug.SetMemoryLimit(-1) != got {
		t.Fatalf("limit=%d budget=%d", limit, got)
	}
}
