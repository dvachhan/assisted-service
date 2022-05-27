package odflvm

import "github.com/openshift/assisted-service/models"

type Config struct {
	ODFLVMCPUPerHost       int64 `envconfig:"ODF_LVM_CPU_Per_Host" default:""`
	ODFLVMMemoryMiBPerHost int64 `envconfig:"ODF_LVM_Memory_MiB_Per_Host" default:""`
}

// count all disks of drive type ssd or hdd
func (o *operator) getValidDiskCount(disks []*models.Disk, installationDiskID string) int64 {
	var countDisks int64

	for _, disk := range disks {
		if (disk.DriveType == SsdDrive || disk.DriveType == HddDrive) && installationDiskID != disk.ID && disk.SizeBytes != 0 {
			countDisks++
		}
	}
	return countDisks
}
