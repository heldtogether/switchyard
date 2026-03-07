package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const defaultSampleInterval = 10 * time.Second

// UsageSummary captures metered usage for a single completed container run.
type UsageSummary struct {
	CPUSeconds      float64
	MemoryGBSeconds float64
	MaxMemoryBytes  uint64
	DurationSeconds float64
}

type usageSample struct {
	at              time.Time
	cpuTotalNS      uint64
	workingSetBytes uint64
}

// TrackContainerUsage polls Docker ContainerStats and integrates CPU + memory usage.
func TrackContainerUsage(
	ctx context.Context,
	docker *client.Client,
	containerID string,
	sampleInterval time.Duration,
) (*UsageSummary, error) {
	if docker == nil {
		return nil, errors.New("docker client is nil")
	}
	if containerID == "" {
		return nil, errors.New("container id is required")
	}
	if sampleInterval <= 0 {
		sampleInterval = defaultSampleInterval
	}

	startedAt := time.Now().UTC()
	var prevCPUNS *uint64
	var prevSample *usageSample
	summary := &UsageSummary{}

	// Begin sample: ensures short-lived jobs have a memory baseline.
	if sample, err := readUsageSample(ctx, docker, containerID); err == nil {
		applyUsageSample(summary, sample, prevSample, &prevCPUNS)
		prevSample = sample
	}

	for {
		select {
		case <-ctx.Done():
			summary.DurationSeconds = time.Since(startedAt).Seconds()
			return summary, ctx.Err()
		default:
		}

		running, inspectErr := isContainerRunning(ctx, docker, containerID)
		if inspectErr != nil {
			summary.DurationSeconds = time.Since(startedAt).Seconds()
			return summary, inspectErr
		}
		if !running {
			break
		}

		timer := time.NewTimer(sampleInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			summary.DurationSeconds = time.Since(startedAt).Seconds()
			return summary, ctx.Err()
		case <-timer.C:
		}

		if sample, err := readUsageSample(ctx, docker, containerID); err == nil {
			applyUsageSample(summary, sample, prevSample, &prevCPUNS)
			prevSample = sample
		}
	}

	// End sample: captures short runs that finish before the first periodic tick.
	if sample, err := readUsageSample(ctx, docker, containerID); err == nil {
		applyUsageSample(summary, sample, prevSample, &prevCPUNS)
		prevSample = sample
	}

	summary.DurationSeconds = time.Since(startedAt).Seconds()
	return summary, nil
}

func readContainerStats(ctx context.Context, docker *client.Client, containerID string) (*container.StatsResponse, time.Time, error) {
	resp, err := docker.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		if err == io.EOF {
			return nil, time.Time{}, err
		}
		return nil, time.Time{}, err
	}
	return &stats, time.Now().UTC(), nil
}

func readUsageSample(ctx context.Context, docker *client.Client, containerID string) (*usageSample, error) {
	stats, sampledAt, err := readContainerStats(ctx, docker, containerID)
	if err != nil {
		return nil, err
	}
	return &usageSample{
		at:              sampledAt,
		cpuTotalNS:      stats.CPUStats.CPUUsage.TotalUsage,
		workingSetBytes: workingSetBytes(stats.MemoryStats.Usage, stats.MemoryStats.Stats),
	}, nil
}

func isContainerRunning(ctx context.Context, docker *client.Client, containerID string) (bool, error) {
	inspect, err := docker.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	if inspect.State == nil {
		return false, nil
	}
	return inspect.State.Running, nil
}

func applyUsageSample(summary *UsageSummary, sample *usageSample, prevSample *usageSample, prevCPUNS **uint64) {
	accumulateCPUDelta(&summary.CPUSeconds, prevCPUNS, sample.cpuTotalNS)
	if sample.workingSetBytes > summary.MaxMemoryBytes {
		summary.MaxMemoryBytes = sample.workingSetBytes
	}
	if prevSample != nil {
		integrateMemoryGBSecondsTrapezoid(
			&summary.MemoryGBSeconds,
			prevSample.workingSetBytes,
			sample.workingSetBytes,
			sample.at.Sub(prevSample.at).Seconds(),
		)
	}
}

func accumulateCPUDelta(acc *float64, prev **uint64, current uint64) {
	if *prev == nil {
		initial := current
		*prev = &initial
		return
	}
	if current < **prev {
		cur := current
		*prev = &cur
		return
	}

	deltaNS := current - **prev
	*acc += float64(deltaNS) / 1e9
	cur := current
	*prev = &cur
}

func workingSetBytes(usage uint64, stats map[string]uint64) uint64 {
	cacheLike := uint64(0)
	if v, ok := stats["inactive_file"]; ok {
		cacheLike = v
	} else if v, ok := stats["cache"]; ok {
		cacheLike = v
	}
	if usage <= cacheLike {
		return 0
	}
	return usage - cacheLike
}

func integrateMemoryGBSeconds(acc *float64, workingSetBytes uint64, intervalSeconds float64) {
	if intervalSeconds <= 0 {
		return
	}
	*acc += (float64(workingSetBytes) / 1e9) * math.Max(0, intervalSeconds)
}

func integrateMemoryGBSecondsTrapezoid(acc *float64, prevWorkingSetBytes uint64, currentWorkingSetBytes uint64, intervalSeconds float64) {
	if intervalSeconds <= 0 {
		return
	}
	prevGB := float64(prevWorkingSetBytes) / 1e9
	currentGB := float64(currentWorkingSetBytes) / 1e9
	*acc += ((prevGB + currentGB) / 2.0) * math.Max(0, intervalSeconds)
}
