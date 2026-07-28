package domain

import "fmt"

type VMSize struct {
	Name    string
	CPU     int
	MemoryMB int
	DiskGB  int
}

type SizeMinimums struct {
	CPU     int
	MemoryMB int
	DiskGB  int
}

var masterMinimums = SizeMinimums{CPU: 2, MemoryMB: 4096, DiskGB: 20}
var workerMinimums = SizeMinimums{CPU: 2, MemoryMB: 2048, DiskGB: 20}

var catalog = map[string]VMSize{
	"cp-small":   {Name: "cp-small", CPU: 2, MemoryMB: 4096, DiskGB: 40},
	"cp-medium":  {Name: "cp-medium", CPU: 4, MemoryMB: 8192, DiskGB: 60},
	"cp-large":   {Name: "cp-large", CPU: 8, MemoryMB: 16384, DiskGB: 100},
	"standard-2": {Name: "standard-2", CPU: 2, MemoryMB: 4096, DiskGB: 60},
	"standard-4": {Name: "standard-4", CPU: 4, MemoryMB: 8192, DiskGB: 100},
	"standard-8": {Name: "standard-8", CPU: 8, MemoryMB: 16384, DiskGB: 200},
	"memory-4":   {Name: "memory-4", CPU: 4, MemoryMB: 16384, DiskGB: 200},
	"memory-8":   {Name: "memory-8", CPU: 8, MemoryMB: 32768, DiskGB: 400},
	"cpu-8":      {Name: "cpu-8", CPU: 8, MemoryMB: 8192, DiskGB: 100},
}

func SizeByName(name string) (VMSize, bool) {
	s, ok := catalog[name]
	return s, ok
}

func AllSizes() []VMSize {
	order := []string{"cp-small", "cp-medium", "cp-large", "standard-2", "standard-4", "standard-8", "memory-4", "memory-8", "cpu-8"}
	out := make([]VMSize, 0, len(order))
	for _, name := range order {
		out = append(out, catalog[name])
	}
	return out
}

func ValidateSize(size VMSize, role NodeRole) error {
	var mins SizeMinimums
	if role == RoleMaster {
		mins = masterMinimums
	} else {
		mins = workerMinimums
	}

	if size.CPU < mins.CPU {
		return ErrSizeBelowMinimum(size.Name, string(role), "cpu", size.CPU, mins.CPU)
	}
	if size.MemoryMB < mins.MemoryMB {
		return ErrSizeBelowMinimum(size.Name, string(role), "memory_mb", size.MemoryMB, mins.MemoryMB)
	}
	if size.DiskGB < mins.DiskGB {
		return ErrSizeBelowMinimum(size.Name, string(role), "disk_gb", size.DiskGB, mins.DiskGB)
	}
	return nil
}

func ResolveSize(sizeName string, cpuOverride, memMBOverride, diskGBOverride int) (VMSize, error) {
	if sizeName == "" && cpuOverride == 0 {
		return VMSize{}, fmt.Errorf("either size name or explicit resources (cpu, memory_mb, disk_gb) must be specified")
	}

	var base VMSize
	if sizeName != "" {
		s, ok := SizeByName(sizeName)
		if !ok {
			return VMSize{}, fmt.Errorf("unknown size %q; run 'proxmox-k3s sizes' to list available sizes", sizeName)
		}
		base = s
	}

	if cpuOverride > 0 {
		base.CPU = cpuOverride
	}
	if memMBOverride > 0 {
		base.MemoryMB = memMBOverride
	}
	if diskGBOverride > 0 {
		base.DiskGB = diskGBOverride
	}

	if base.Name == "" {
		base.Name = fmt.Sprintf("custom(%d cpu/%d MB/%d GB)", base.CPU, base.MemoryMB, base.DiskGB)
	}

	return base, nil
}
