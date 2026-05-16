package probe

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chirs/stordiag/internal/driver"
)

type diskStat struct {
	device     string
	reads      float64
	readMs     float64
	writes     float64
	writeMs    float64
	ioInFlight float64
	ioMs       float64
}

type sysSnapshot struct {
	disk   map[string]diskStat
	cpuIO  float64 // iowait from /proc/stat
	memAvailKB  int64
	pressureSomeCPU float64
	pressureSomeIO  float64
}

func ProbeSystem(ctx context.Context, drv driver.Driver) *LayerReport {
	r := &LayerReport{Layer: LayerSystem}

	if runtime.GOOS != "linux" {
		r.Add("system", "N/A", 0, "system layer only supported on Linux", "")
		return r
	}

	// --- Disk IO stats ---
	s1, err := readSysSnapshot()
	if err != nil {
		r.Error("disk_snapshot_1", 0, err)
	} else {
		select {
		case <-ctx.Done():
			r.Error("disk_snapshot", 0, ctx.Err())
			return r
		case <-time.After(1 * time.Second):
		}
		s2, err := readSysSnapshot()
		if err != nil {
			r.Error("disk_snapshot_2", 0, err)
		} else {
			reportDiskIO(r, s1, s2)
		}
	}

	// --- Memory ---
	if s1 != nil {
		reportMemory(r, s1)
	}

	// --- Pressure Stall ---
	reportPressure(r)

	// --- CPU iowait ---
	if s1 != nil {
		reportCPU(r, s1)
	}

	return r
}

func readSysSnapshot() (*sysSnapshot, error) {
	s := &sysSnapshot{disk: make(map[string]diskStat)}

	// /proc/diskstats
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, fmt.Errorf("read /proc/diskstats: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		// major minor device reads readMs ...
		if len(line) < 14 {
			continue
		}
		device := line[2]
		// skip partitions (contain digits at end) and ram/loop devices
		if isVirtualDevice(device) {
			continue
		}
		reads, _ := strconv.ParseFloat(line[3], 64)
		readMs, _ := strconv.ParseFloat(line[6], 64)
		writes, _ := strconv.ParseFloat(line[7], 64)
		writeMs, _ := strconv.ParseFloat(line[10], 64)
		ioInFlight, _ := strconv.ParseFloat(line[11], 64) // #IOs currently in flight
		ioMs, _ := strconv.ParseFloat(line[12], 64)

		s.disk[device] = diskStat{
			device: device, reads: reads, readMs: readMs,
			writes: writes, writeMs: writeMs,
			ioInFlight: ioInFlight, ioMs: ioMs,
		}
	}

	// /proc/stat -> iowait (line 0, field 5)
	if statData, err := os.ReadFile("/proc/stat"); err == nil {
		for _, line := range strings.Split(string(statData), "\n") {
			f := strings.Fields(line)
			if len(f) >= 6 && f[0] == "cpu" {
				s.cpuIO, _ = strconv.ParseFloat(f[5], 64)
				break
			}
		}
	}

	// /proc/meminfo -> MemAvailable
	if memData, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(memData), "\n") {
			if strings.HasPrefix(line, "MemAvailable:") {
				f := strings.Fields(line)
				if len(f) >= 2 {
					s.memAvailKB, _ = strconv.ParseInt(f[1], 10, 64)
				}
				break
			}
		}
	}

	// /proc/pressure
	s.pressureSomeCPU = readPressure("/proc/pressure/cpu", "some")
	s.pressureSomeIO = readPressure("/proc/pressure/io", "some")

	return s, nil
}

func readPressure(path, prefix string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix+" ") {
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "avg60=") {
					v, _ := strconv.ParseFloat(field[6:], 64)
					return v
				}
			}
		}
	}
	return -1
}

func isVirtualDevice(name string) bool {
	if strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "zram") {
		return true
	}
	// skip partitions: nvme0n1p1, sda1, etc.
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] >= '0' && name[i] <= '9' {
			continue
		}
		return name[i] == 'p' && i > 0 && name[i-1] >= '0' && name[i-1] <= '9'
	}
	return false
}

func reportDiskIO(r *LayerReport, s1, s2 *sysSnapshot) {
	elapsed := 1.0 // 1 second

	for device, d2 := range s2.disk {
		d1, ok := s1.disk[device]
		if !ok {
			continue
		}

		reads := d2.reads - d1.reads
		writes := d2.writes - d1.writes
		iops := (reads + writes) / elapsed
		readLat := safeDiv(d2.readMs-d1.readMs, reads)
		writeLat := safeDiv(d2.writeMs-d1.writeMs, writes)
		avgWait := safeDiv(d2.ioMs-d1.ioMs, reads+writes)

		r.Add("disk_"+device, "OK", time.Duration(avgWait)*time.Millisecond,
			fmt.Sprintf("iops=%.0f r_await=%.1fms w_await=%.1fms queue=%.0f",
				iops, readLat, writeLat, d2.ioInFlight),
			fmt.Sprintf("/proc/diskstats delta over 1s"))
	}
}

func reportMemory(r *LayerReport, s *sysSnapshot) {
	availMB := float64(s.memAvailKB) / 1024
	status := "OK"
	if availMB < 500 {
		status = "WARN"
	}
	if availMB < 100 {
		status = "FAIL"
	}
	r.Add("memory_available", status, 0,
		fmt.Sprintf("%.0f MiB", availMB), "/proc/meminfo MemAvailable")
}

func reportCPU(r *LayerReport, s *sysSnapshot) {
	r.Add("cpu_iowait", "OK", 0,
		fmt.Sprintf("iowait=%.1f tick", s.cpuIO), "/proc/stat cpu[5]")
}

func reportPressure(r *LayerReport) {
	// CPU pressure
	if cpu := readPressure("/proc/pressure/cpu", "some"); cpu >= 0 {
		status := "OK"
		if cpu > 10 {
			status = "WARN"
		}
		if cpu > 50 {
			status = "FAIL"
		}
		r.Add("pressure_cpu", status, 0,
			fmt.Sprintf("avg60=%.1f%%", cpu), "/proc/pressure/cpu some avg60")
	}

	// IO pressure
	if io := readPressure("/proc/pressure/io", "some"); io >= 0 {
		status := "OK"
		if io > 10 {
			status = "WARN"
		}
		if io > 50 {
			status = "FAIL"
		}
		r.Add("pressure_io", status, 0,
			fmt.Sprintf("avg60=%.1f%%", io), "/proc/pressure/io some avg60")
	}
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return math.Abs(a) / b
}

// ProbeSystemBasic implements a quick lightweight version
func ProbeSystemBasic(ctx context.Context, drv driver.Driver) *LayerReport {
	r := &LayerReport{Layer: LayerSystem}

	// quick disk stat (single read, no delta)
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		r.Error("disk_usage", 0, fmt.Errorf("/proc/diskstats: %w", err))
	} else {
		var count int
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.Fields(scanner.Text())
			if len(line) >= 14 && !isVirtualDevice(line[2]) {
				count++
			}
		}
		r.Add("disk_devices", "OK", 0, fmt.Sprintf("%d block devices", count), "")
	}

	return r
}
