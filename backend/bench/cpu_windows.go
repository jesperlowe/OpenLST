//go:build windows

package bench

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// platformCPUName læser processorens navn fra registry (kræver ikke admin).
func platformCPUName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue("ProcessorNameString")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}
