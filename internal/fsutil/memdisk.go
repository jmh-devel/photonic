package fsutil

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// MemDisk manages a temporary memory-based filesystem
type MemDisk struct {
	MountPoint string
	Size       int64 // Size in MB
	mounted    bool
	logger     *slog.Logger
}

// GetSystemMemory returns available memory in MB
func GetSystemMemory() (int64, error) {
	// Try to read /proc/meminfo for more accurate available memory
	content, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemAvailable:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						return kb / 1024, nil // Convert KB to MB
					}
				}
			}
		}
	}

	// Fallback to syscall if /proc/meminfo parsing fails
	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		return 0, err
	}

	// Available memory in MB (conservative estimate using free memory)
	availableBytes := int64(sysinfo.Freeram) * int64(sysinfo.Unit)
	availableMB := availableBytes / (1024 * 1024)

	return availableMB, nil
}

// EstimateDatasetSize estimates the space needed for RAW file processing
// Returns estimated size in MB
func EstimateDatasetSize(rawFiles []string) (int64, error) {
	if len(rawFiles) == 0 {
		return 0, nil
	}

	// Sample a few files to estimate average size
	sampleSize := len(rawFiles)
	if sampleSize > 5 {
		sampleSize = 5 // Sample max 5 files for estimation
	}

	var totalSampleSize int64
	for i := 0; i < sampleSize; i++ {
		if stat, err := os.Stat(rawFiles[i]); err == nil {
			totalSampleSize += stat.Size()
		}
	}

	if totalSampleSize == 0 {
		return 0, fmt.Errorf("could not determine file sizes")
	}

	avgFileSize := totalSampleSize / int64(sampleSize)

	// Estimate: RAW file + converted JPEG (assume JPEG is ~20% of RAW size)
	estimatedTotalBytes := int64(len(rawFiles)) * avgFileSize * 12 / 10 // 1.2x for safety margin
	estimatedMB := estimatedTotalBytes / (1024 * 1024)

	return estimatedMB, nil
}

// ShouldUseMemDisk determines if we should use memory disk based on available RAM and dataset size
func ShouldUseMemDisk(rawFiles []string, logger *slog.Logger) (bool, int64, error) {
	if len(rawFiles) == 0 {
		return false, 0, nil
	}

	availableRAM, err := GetSystemMemory()
	if err != nil {
		if logger != nil {
			logger.Debug("failed to get system memory info", "error", err)
		}
		return false, 0, nil
	}

	datasetSizeMB, err := EstimateDatasetSize(rawFiles)
	if err != nil {
		if logger != nil {
			logger.Debug("failed to estimate dataset size", "error", err)
		}
		return false, 0, nil
	}

	// More practical approach: use memory disk if dataset is < 50% of available RAM
	// and we have at least 512MB free RAM remaining
	memDiskSize := datasetSizeMB + 100 // Add 100MB buffer
	minFreeRAM := int64(512)           // Require at least 512MB free (reduced from 1GB)

	if logger != nil {
		logger.Info("memory disk feasibility check",
			"available_ram_mb", availableRAM,
			"estimated_dataset_mb", datasetSizeMB,
			"required_memdisk_mb", memDiskSize,
			"min_free_ram_mb", minFreeRAM,
		)
	}

	if memDiskSize < availableRAM/2 && (availableRAM-memDiskSize) > minFreeRAM {
		return true, memDiskSize, nil
	}

	if logger != nil {
		logger.Info("memory disk not recommended",
			"reason", "insufficient RAM or dataset too large",
			"available_ram_mb", availableRAM,
			"required_mb", memDiskSize,
		)
	}

	return false, 0, nil
}

// NewMemDisk creates a new memory disk with the specified size
func NewMemDisk(sizeMB int64, logger *slog.Logger) (*MemDisk, error) {
	// Create unique mount point
	mountPoint := filepath.Join("/tmp", fmt.Sprintf("photonic-memdisk-%d", os.Getpid()))

	md := &MemDisk{
		MountPoint: mountPoint,
		Size:       sizeMB,
		logger:     logger,
	}

	if err := md.Mount(); err != nil {
		// If mount fails (e.g., no root permissions), try to use existing tmpfs
		if logger != nil {
			logger.Info("tmpfs mount failed, checking for existing tmpfs mount", "error", err)
		}

		// Check if /tmp is already on tmpfs
		if md.isTmpfsAvailable() {
			// Use /tmp directly since it's already in memory
			md.mounted = true
			if logger != nil {
				logger.Info("using existing tmpfs at /tmp for memory disk", "mount_point", mountPoint)
			}
			// Just create the directory in existing tmpfs
			if err := os.MkdirAll(md.MountPoint, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory in tmpfs: %v", err)
			}
			return md, nil
		}

		return nil, err
	}

	return md, nil
}

// isTmpfsAvailable checks if /tmp is mounted on tmpfs
func (md *MemDisk) isTmpfsAvailable() bool {
	// Read /proc/mounts to check if /tmp is tmpfs
	content, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "/tmp" && fields[2] == "tmpfs" {
			if md.logger != nil {
				md.logger.Info("detected existing tmpfs mount", "mount_point", "/tmp", "filesystem", fields[2])
			}
			return true
		}
	}

	return false
} // Mount creates and mounts the tmpfs
func (md *MemDisk) Mount() error {
	// Create mount point directory
	if err := os.MkdirAll(md.MountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %v", err)
	}

	// Mount tmpfs
	sizeOpt := fmt.Sprintf("size=%dM", md.Size)
	cmd := exec.Command("mount", "-t", "tmpfs", "-o", sizeOpt, "tmpfs", md.MountPoint)

	if md.logger != nil {
		md.logger.Info("mounting memory disk",
			"mount_point", md.MountPoint,
			"size_mb", md.Size,
		)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(md.MountPoint) // Clean up directory
		return fmt.Errorf("failed to mount tmpfs: %v, output: %s", err, string(output))
	}

	md.mounted = true

	if md.logger != nil {
		md.logger.Info("memory disk mounted successfully",
			"mount_point", md.MountPoint,
			"size_mb", md.Size,
		)
	}

	return nil
}

// Cleanup unmounts and removes the memory disk
func (md *MemDisk) Cleanup() error {
	if !md.mounted {
		return nil
	}

	if md.logger != nil {
		md.logger.Info("cleaning up memory disk", "mount_point", md.MountPoint)
	}

	// Unmount
	cmd := exec.Command("umount", md.MountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		if md.logger != nil {
			md.logger.Warn("failed to unmount memory disk",
				"error", err,
				"output", string(output),
				"mount_point", md.MountPoint,
			)
		}
		// Continue with cleanup even if unmount fails
	}

	// Remove mount point directory
	if err := os.RemoveAll(md.MountPoint); err != nil {
		if md.logger != nil {
			md.logger.Warn("failed to remove mount point directory",
				"error", err,
				"mount_point", md.MountPoint,
			)
		}
	}

	md.mounted = false

	if md.logger != nil {
		md.logger.Info("memory disk cleanup completed", "mount_point", md.MountPoint)
	}

	return nil
}

// GetPath returns the mount point path for storing temporary files
func (md *MemDisk) GetPath() string {
	return md.MountPoint
}

// IsActive returns true if the memory disk is mounted and active
func (md *MemDisk) IsActive() bool {
	return md.mounted
}
