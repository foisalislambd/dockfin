//go:build !linux

package main

func sampleHostMetrics() (cpu float64, memUsed, memTotal, diskUsed, diskTotal int64) {
	return 0, 0, 0, 0, 0
}
