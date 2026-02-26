package executor

import (
	"fmt"
	"time"
)

// BuildSystemEnv creates system-managed environment variables that are
// automatically injected into every job container. These variables use
// the SWITCHYARD_ prefix and cannot be overridden by users.
//
// System environment variables include:
//   - Job identity and timing information
//   - Execution context (executor type, image details)
//   - Storage and output paths
//   - System version and API URL
//   - Resource limits (if configured)
func BuildSystemEnv(spec RunSpec) []string {
	env := []string{
		// Core job identity
		fmt.Sprintf("SWITCHYARD_JOB_ID=%s", spec.JobID),
		fmt.Sprintf("SWITCHYARD_JOB_CREATED_AT=%s", spec.CreatedAt.Format(time.RFC3339)),
		fmt.Sprintf("SWITCHYARD_JOB_TIMEOUT=%d", int(spec.Timeout.Seconds())),

		// Execution context
		fmt.Sprintf("SWITCHYARD_EXECUTOR_TYPE=%s", spec.ExecutorType),
		fmt.Sprintf("SWITCHYARD_IMAGE=%s", spec.Image),

		// Storage & outputs
		fmt.Sprintf("SWITCHYARD_OUTPUTS_DIR=/outputs"),
		fmt.Sprintf("SWITCHYARD_BUCKET=%s", spec.Bucket),

		// System info
		fmt.Sprintf("SWITCHYARD_VERSION=%s", spec.SwitchyardVersion),
	}

	// Optional fields - only add if present
	if spec.ImageDigest != "" {
		env = append(env, fmt.Sprintf("SWITCHYARD_IMAGE_DIGEST=%s", spec.ImageDigest))
	}

	if spec.ArtefactPrefix != "" {
		env = append(env, fmt.Sprintf("SWITCHYARD_ARTEFACT_PREFIX=%s", spec.ArtefactPrefix))
	}

	if spec.APIBaseURL != "" {
		env = append(env, fmt.Sprintf("SWITCHYARD_API_URL=%s", spec.APIBaseURL))
	}

	if spec.CPU != "" {
		env = append(env, fmt.Sprintf("SWITCHYARD_CPU_LIMIT=%s", spec.CPU))
	}

	if spec.Memory != "" {
		env = append(env, fmt.Sprintf("SWITCHYARD_MEMORY_LIMIT=%s", spec.Memory))
	}
	if spec.GPUCount > 0 {
		env = append(env, fmt.Sprintf("SWITCHYARD_GPU_COUNT=%d", spec.GPUCount))
	}

	// Add user environment variables after system ones
	// This ensures system variables cannot be overridden
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}
