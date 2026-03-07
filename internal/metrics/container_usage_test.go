package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccumulateCPUDelta(t *testing.T) {
	var acc float64
	var prev *uint64

	accumulateCPUDelta(&acc, &prev, 1_000_000_000)
	require.Equal(t, 0.0, acc)

	accumulateCPUDelta(&acc, &prev, 2_500_000_000)
	require.InDelta(t, 1.5, acc, 1e-9)

	// Non-monotonic should be ignored for delta and reset baseline.
	accumulateCPUDelta(&acc, &prev, 2_000_000_000)
	require.InDelta(t, 1.5, acc, 1e-9)

	accumulateCPUDelta(&acc, &prev, 3_000_000_000)
	require.InDelta(t, 2.5, acc, 1e-9)
}

func TestWorkingSetBytes_CgroupV2InactiveFile(t *testing.T) {
	got := workingSetBytes(1_000, map[string]uint64{
		"inactive_file": 300,
		"cache":         200,
	})
	require.Equal(t, uint64(700), got)
}

func TestWorkingSetBytes_CgroupV1CacheFallback(t *testing.T) {
	got := workingSetBytes(2_000, map[string]uint64{
		"cache": 400,
	})
	require.Equal(t, uint64(1600), got)
}

func TestWorkingSetBytes_NoCacheField(t *testing.T) {
	got := workingSetBytes(2_000, map[string]uint64{})
	require.Equal(t, uint64(2000), got)
}

func TestIntegrateMemoryGBSeconds(t *testing.T) {
	var acc float64
	oneGB := uint64(1_000_000_000)

	integrateMemoryGBSeconds(&acc, oneGB, 10)
	require.InDelta(t, 10.0, acc, 1e-9)

	integrateMemoryGBSeconds(&acc, oneGB/2, 2)
	require.InDelta(t, 11.0, acc, 1e-9)
}

func TestIntegrateMemoryGBSecondsTrapezoid(t *testing.T) {
	var acc float64
	integrateMemoryGBSecondsTrapezoid(&acc, 1_000_000_000, 3_000_000_000, 2)
	// Average = 2GB for 2s => 4 GB-s.
	require.InDelta(t, 4.0, acc, 1e-9)
}

func TestApplyUsageSample_TracksMaxAndIntegrates(t *testing.T) {
	s := &UsageSummary{}
	var prev *uint64

	start := time.Now().UTC()
	sample1 := &usageSample{
		at:              start,
		cpuTotalNS:      1_000_000_000,
		workingSetBytes: 1_500_000_000,
	}

	applyUsageSample(s, sample1, nil, &prev)
	require.InDelta(t, 0.0, s.CPUSeconds, 1e-9)
	require.Equal(t, uint64(1_500_000_000), s.MaxMemoryBytes)

	sample2 := &usageSample{
		at:              start.Add(10 * time.Second),
		cpuTotalNS:      3_000_000_000,
		workingSetBytes: 900_000_000,
	}

	applyUsageSample(s, sample2, sample1, &prev)
	require.InDelta(t, 2.0, s.CPUSeconds, 1e-9)
	require.InDelta(t, 12.0, s.MemoryGBSeconds, 1e-9)
	require.Equal(t, uint64(1_500_000_000), s.MaxMemoryBytes)
}
