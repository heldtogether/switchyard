package billing

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyKeyGeneration(t *testing.T) {
	workspaceID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	runID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	jobID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	cpu := BuildCPUMeterIdempotencyKey(workspaceID, runID, jobID)
	mem := BuildMemoryMeterIdempotencyKey(workspaceID, runID, jobID)
	gpu := BuildGPUMeterIdempotencyKey(workspaceID, runID, jobID)

	require.Equal(t, "org_aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa_run_bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb_job_cccccccc-cccc-cccc-cccc-cccccccccccc_meter_cpu_seconds", cpu)
	require.Equal(t, "org_aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa_run_bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb_job_cccccccc-cccc-cccc-cccc-cccccccccccc_meter_memory_gb_seconds", mem)
	require.Equal(t, "org_aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa_run_bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb_job_cccccccc-cccc-cccc-cccc-cccccccccccc_meter_gpu_seconds", gpu)
}
