package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func sampleHostMetrics() (cpu float64, memUsed, memTotal, diskUsed, diskTotal int64) {
	u1, t1 := readCPU()
	time.Sleep(200 * time.Millisecond)
	u2, t2 := readCPU()
	if dt := t2 - t1; dt > 0 {
		cpu = 100 * float64(u2-u1) / float64(dt)
	}
	memTotal, memUsed = readMem()
	diskTotal, diskUsed = readDisk("/")
	return
}

func readCPU() (used, total uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	line, _, _ := strings.Cut(string(data), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0
	}
	user, _ := strconv.ParseUint(fields[1], 10, 64)
	nice, _ := strconv.ParseUint(fields[2], 10, 64)
	system, _ := strconv.ParseUint(fields[3], 10, 64)
	idle, _ := strconv.ParseUint(fields[4], 10, 64)
	used = user + nice + system
	total = used + idle
	return
}

func readMem() (total, used int64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var avail int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		n, _ := strconv.ParseInt(fields[1], 10, 64)
		n *= 1024
		switch fields[0] {
		case "MemTotal:":
			total = n
		case "MemAvailable:":
			avail = n
		}
	}
	if total > avail {
		used = total - avail
	}
	return total, used
}

func readDisk(path string) (total, used int64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	total = int64(st.Blocks) * int64(st.Bsize)
	free := int64(st.Bavail) * int64(st.Bsize)
	if total > free {
		used = total - free
	}
	return total, used
}
