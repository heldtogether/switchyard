package worker

import "testing"

func TestCountGPULines(t *testing.T) {
	input := "GPU 0: NVIDIA A100-SXM4-40GB (UUID: GPU-123)\nGPU 1: NVIDIA A100-SXM4-40GB (UUID: GPU-456)\n"
	if got := countGPULines(input); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := countGPULines(""); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}
