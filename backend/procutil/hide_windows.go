//go:build windows

package procutil

import (
	"log"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// Hide forhindrer at underprocessen åbner et konsolvindue.
func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

var (
	killJobOnce sync.Once
	killJob     windows.Handle
)

// KillWithParent lægger en startet proces i et Job Object, der lukkes når
// OpenLST afslutter. Så dør piper/whisper altid sammen med OpenLST – også hvis
// OpenLST bliver dræbt.
func KillWithParent(cmd *exec.Cmd) {
	killJobOnce.Do(func() {
		j, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			log.Printf("job object: %v", err)
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(j, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			log.Printf("job object: %v", err)
			windows.CloseHandle(j)
			return
		}
		killJob = j // holdes åben, til processen afslutter
	})
	if killJob == 0 || cmd.Process == nil {
		return
	}
	if err := assign(killJob, cmd.Process.Pid); err != nil {
		log.Printf("job object: %v", err)
	}
}

func assign(job windows.Handle, pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

// watchPeak lægger processen i sit eget job og læser peak-hukommelsen bagefter.
// Jobbet bevarer tallet efter processen er afsluttet.
func watchPeak(cmd *exec.Cmd) func() uint64 {
	j, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() uint64 { return 0 }
	}
	if err := assign(j, cmd.Process.Pid); err != nil {
		windows.CloseHandle(j)
		return func() uint64 { return 0 }
	}
	return func() uint64 {
		defer windows.CloseHandle(j)
		var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		if err := windows.QueryInformationJobObject(j, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
			return 0
		}
		return uint64(info.PeakProcessMemoryUsed)
	}
}
