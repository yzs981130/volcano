package lease

const (
	// The key of renewing annotation in the PodGroup.
	PodGroupRenewingAnnoKey = "scheduling.yzs981130.io/renewing"

	PodGroupRenewingOngoing = "true"
	PodGroupRenewingNotOngoing = "false"

	// The key of renewing result annotation in the PodGroup.
	PodGroupRenewingResultAnnoKey = "scheduling.yzs981130.io/renewing-result"

	// PodGroupRenewingSucceeded means the renewal succeeded.
	PodGroupRenewingSucceeded = "succeeded"

	// PodGroupRenewingFailed means the renewal failed.
	PodGroupRenewingFailed = "failed"
)
