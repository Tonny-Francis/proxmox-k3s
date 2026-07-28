package domain

import (
	"testing"
)

func TestSizeByName(t *testing.T) {
	tests := []struct {
		name     string
		wantCPU  int
		wantMem  int
		wantDisk int
	}{
		{"cp-small", 2, 4096, 40},
		{"cp-medium", 4, 8192, 60},
		{"standard-4", 4, 8192, 100},
		{"memory-8", 8, 32768, 400},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, ok := SizeByName(tc.name)
			if !ok {
				t.Fatalf("size %q not found", tc.name)
			}
			if s.CPU != tc.wantCPU {
				t.Errorf("CPU: got %d, want %d", s.CPU, tc.wantCPU)
			}
			if s.MemoryMB != tc.wantMem {
				t.Errorf("MemoryMB: got %d, want %d", s.MemoryMB, tc.wantMem)
			}
			if s.DiskGB != tc.wantDisk {
				t.Errorf("DiskGB: got %d, want %d", s.DiskGB, tc.wantDisk)
			}
		})
	}
}

func TestSizeByNameUnknown(t *testing.T) {
	_, ok := SizeByName("nonexistent-size")
	if ok {
		t.Fatal("expected false for unknown size")
	}
}

func TestValidateSize_Master_AboveMinimum(t *testing.T) {
	size := VMSize{Name: "cp-medium", CPU: 4, MemoryMB: 8192, DiskGB: 60}
	if err := ValidateSize(size, RoleMaster); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateSize_Master_BelowCPU(t *testing.T) {
	size := VMSize{Name: "tiny", CPU: 1, MemoryMB: 4096, DiskGB: 40}
	if err := ValidateSize(size, RoleMaster); err == nil {
		t.Error("expected error for CPU below minimum, got nil")
	}
}

func TestValidateSize_Master_BelowMemory(t *testing.T) {
	size := VMSize{Name: "low-mem", CPU: 2, MemoryMB: 2048, DiskGB: 40}
	if err := ValidateSize(size, RoleMaster); err == nil {
		t.Error("expected error for memory below minimum, got nil")
	}
}

func TestValidateSize_Worker_MinimumExact(t *testing.T) {
	size := VMSize{Name: "min-worker", CPU: 2, MemoryMB: 2048, DiskGB: 20}
	if err := ValidateSize(size, RoleWorker); err != nil {
		t.Errorf("expected no error for minimum worker size, got: %v", err)
	}
}

func TestResolveSize_ByName(t *testing.T) {
	s, err := ResolveSize("standard-4", 0, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.CPU != 4 || s.MemoryMB != 8192 || s.DiskGB != 100 {
		t.Errorf("unexpected size: %+v", s)
	}
}

func TestResolveSize_WithOverride(t *testing.T) {
	s, err := ResolveSize("standard-4", 0, 0, 400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DiskGB != 400 {
		t.Errorf("disk override not applied: got %d, want 400", s.DiskGB)
	}
	if s.CPU != 4 {
		t.Errorf("CPU should be from catalog: got %d, want 4", s.CPU)
	}
}

func TestResolveSize_UnknownName(t *testing.T) {
	_, err := ResolveSize("does-not-exist", 0, 0, 0)
	if err == nil {
		t.Error("expected error for unknown size name")
	}
}

func TestAllSizes_CompleteCatalog(t *testing.T) {
	sizes := AllSizes()
	if len(sizes) == 0 {
		t.Fatal("expected non-empty catalog")
	}

	expected := []string{"cp-small", "cp-medium", "cp-large", "standard-2", "standard-4", "standard-8", "memory-4", "memory-8", "cpu-8"}
	if len(sizes) != len(expected) {
		t.Errorf("expected %d sizes, got %d", len(expected), len(sizes))
	}
}
