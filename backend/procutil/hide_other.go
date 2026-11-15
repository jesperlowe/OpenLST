//go:build !windows

package procutil

import (
	"os/exec"
	"runtime"
	"syscall"
)

// Hide gør intet uden for Windows.
func Hide(cmd *exec.Cmd) {}

// KillWithParent gør intet uden for Windows.
func KillWithParent(cmd *exec.Cmd) {}

func watchPeak(cmd *exec.Cmd) func() uint64 {
	return func() uint64 {
		if cmd.ProcessState == nil {
			return 0
		}
		ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		if !ok {
			return 0
		}
		if runtime.GOOS == "darwin" {
			return uint64(ru.Maxrss) // bytes på macOS
		}
		return uint64(ru.Maxrss) * 1024 // KiB på Linux
	}
}
