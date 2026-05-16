package probe

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/chirs/stordiag/internal/driver"
)

func ProbeFilesystem(ctx context.Context, drv driver.Driver) *LayerReport {
	r := &LayerReport{Layer: LayerSystem}

	if runtime.GOOS != "linux" {
		r.Add("filesystem", "N/A", 0, "only supported on Linux", "")
		return r
	}

	path := drv.String()
	if drv.Type() != "posix" {
		r.Add("filesystem", "N/A", 0, "only POSIX driver supported", "")
		return r
	}

	// Filesystem type from statfs
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		r.Add("fs_type", "WARN", 0,
			fmt.Sprintf("statfs failed: %v", err), "")
	} else {
		fsType := fstypeName(stat.Type)
		r.Add("fs_type", "OK", 0,
			fsType, fmt.Sprintf("fstype=0x%x", stat.Type))
	}

	// Disk usage
	if err := syscall.Statfs(path, &stat); err == nil {
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		avail := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		pct := float64(0)
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}

		r.Add("fs_usage", "OK", 0,
			fmt.Sprintf("%.0f%% used (%s / %s)",
				pct, formatBytes(int64(used)), formatBytes(int64(total))),
			fmt.Sprintf("/proc/mounts avail=%s", formatBytes(int64(avail))))

		// inodes
		totalInodes := stat.Files
		freeInodes := stat.Ffree
		usedInodes := totalInodes - freeInodes
		inodePct := float64(0)
		if totalInodes > 0 {
			inodePct = float64(usedInodes) / float64(totalInodes) * 100
		}
		r.Add("fs_inodes", "OK", 0,
			fmt.Sprintf("%.0f%% used (%d / %d)",
				inodePct, usedInodes, totalInodes), "")
	}

	// Block device rotational detection (SSD vs HDD)
	detectBlockRotational(r, path)

	return r
}

func detectBlockRotational(r *LayerReport, path string) {
	// find block device for path from /proc/mounts
	mounts, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(mounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mountPoint := fields[1]
		mountPoint = strings.ReplaceAll(mountPoint, "\\040", " ")
		if mountPoint != path && !strings.HasPrefix(path, mountPoint+"/") {
			continue
		}
		device := fields[0]
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		// strip /dev/ prefix
		devName := strings.TrimPrefix(device, "/dev/")
		// get base device (strip partition number)
		baseDev := strings.TrimRight(devName, "0123456789")
		if strings.HasSuffix(baseDev, "p") && len(baseDev) > 1 {
			baseDev = baseDev[:len(baseDev)-1]
		}
		if baseDev == "" {
			continue
		}

		data, err := os.ReadFile("/sys/block/" + baseDev + "/queue/rotational")
		if err != nil {
			continue
		}
		rot := strings.TrimSpace(string(data))
		if rot == "0" {
			r.Add("block_device", "OK", 0,
				device+" (SSD)", "")
		} else if rot == "1" {
			r.Add("block_device", "OK", 0,
				device+" (HDD)", "")
		}
		return
	}
}

func fstypeName(fstype int64) string {
	switch fstype {
	case 0xef53:
		return "ext4"
	case 0x58465342:
		return "xfs"
	case 0x6969:
		return "nfs"
	case 0x01021994:
		return "tmpfs"
	case 0x9123683e:
		return "btrfs"
	case 0x00010232:
		return "cifs"
	case 0x65735546:
		return "fuse"
	case 0x73717368:
		return "squashfs"
	case 0x72b6:
		return "jffs2"
	case 0x2fc12fc1:
		return "zfs"
	case 0x24dc0191:
		return "overlay"
	case 0x4d44:
		return "msdos"
	case 0x7c0:
		return "vfat"
	case 0x52654973:
		return "reiserfs"
	case 0x858458f6:
		return "ramfs"
	case 0x6165676c:
		return "eleeprom"
	case 0x00011954:
		return "ufs"
	case 0x19970726:
		return "cramfs"
	default:
		return fmt.Sprintf("0x%x", fstype)
	}
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
