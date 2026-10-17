package memorybudget

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// Configure оставляет 30% cgroup-лимита для SSHFS, стеков и памяти вне Go heap.
// Это мягкая граница GC; живые данные отдельно ограничены parser/writer.
// Явный меньший GOMEMLIMIT сохраняется, больший ограничивается cgroup.
func Configure() int64 {
	current := debug.SetMemoryLimit(-1)
	for _, path := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		limit, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil || limit <= 0 || limit >= 1<<60 {
			continue
		}
		budget := limit / 10 * 7
		if budget < current {
			debug.SetMemoryLimit(budget)
			return budget
		}
		return current
	}
	return current
}
