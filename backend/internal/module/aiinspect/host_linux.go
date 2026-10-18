//go:build linux

package aiinspect

import (
	"fmt"
	"os"
	"syscall"
)

func readCPU() (idle, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	var usr, nic, sys, idleT, iowait, irq, sirq, steal uint64
	if _, err := fmt.Sscanf(string(b), "cpu %d %d %d %d %d %d %d %d", &usr, &nic, &sys, &idleT, &iowait, &irq, &sirq, &steal); err != nil {
		return 0, 0, false
	}
	_ = usr
	return idleT + iowait, usr + nic + sys + idleT + iowait + irq + sirq + steal, true
}

func readMem() (float64, bool) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	var total, avail uint64
	if _, err := fmt.Sscanf(string(b), "MemTotal: %d kB\nMemFree: %d", &total, &avail); err != nil {
		return 0, false
	}
	if total == 0 {
		return 0, false
	}
	return (1 - float64(avail)/float64(total)) * 100, true
}

func readDisk() (float64, bool) {
	var s syscall.Statfs_t
	if err := syscall.Statfs("/", &s); err != nil {
		return 0, false
	}
	if s.Blocks == 0 {
		return 0, false
	}
	used := float64(s.Blocks - s.Bavail)
	return used / float64(s.Blocks) * 100, true
}

func readUptime() (float64, bool) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	var up float64
	if _, err := fmt.Sscanf(string(b), "%f", &up); err != nil {
		return 0, false
	}
	return up, true
}
