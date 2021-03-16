package lease

import (
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

var SpotResource int = 0

func (lp *leasePlugin) FlushLeaseJobs(ssn *framework.Session) {
	requireResourceList, requireLeaseList, maximumLeaseList, inBlockList, remainingResourceList, _ := lp.LeaseInfoCollection(ssn)
	if len(requireLeaseList) > 0 {
		_, solutionMatrix := lp.solver.JobSelection(requireResourceList, requireLeaseList, maximumLeaseList, remainingResourceList)
		for idx, job := range inBlockList {
			jAttr := lp.jobAttrs[job.UID]
			jAttr.cacheSolution = solutionMatrix[idx]
		}
	}
}

func (lp *leasePlugin) FlushEventJobs(ssn *framework.Session) {
	eventJobs := []*api.JobInfo{}
	runnableJobs := []*api.JobInfo{}
	for _, job := range ssn.Jobs {
		if !isFinishedJob(job) {
			if _, found := lp.jobAttrs[job.UID]; !found {
				eventJobs = append(eventJobs, job)
			} else {
				runnableJobs = append(runnableJobs, job)
			}
		}
	}

	requireResourceList, requireLeaseList, maximumLeaseList, _, globalCacheSolution := lp.AbstractLeaseWithCacheList(runnableJobs)
	maxLen := len(maximumLeaseList)
	curTime := 0 // TODO
	eventGuaranteeResource := 0
	inLeaseRemainingTime := curLeaseIndex*leaseTerm - curTime
	for _, job := range eventJobs {
		jAttr := &jobAttr{
			cacheSolution:          nil,
			emergence:              0,
			sloPrioirity:           acceptedSLO,
			occupyLease:            0,
			forceGuarantee:         0,
			stillRequireLease:      0,
			requireMipOptimization: 1,
			reqGPU:                 int(getJobGPUReq(job)),
		}

		maximumLease := 0 // TODO
		requireResourceList = append(requireResourceList, jAttr.reqGPU)
		requireLeaseList = append(requireLeaseList, jAttr.stillRequireLease)
		maximumLeaseList = append(maximumLeaseList, maximumLease)
		if eventGuaranteeResource <= SpotResource {
			if jAttr.remainingTime <= inLeaseRemainingTime {
				jAttr.forceGuarantee = 1
				eventGuaranteeResource += jAttr.reqGPU
			}
			jAttr.requireMipOptimization = 0
		}
		maxLen = len(maximumLeaseList)

		if jAttr.requireMipOptimization == 0 {
			feasible := false
			// var existingSolution []int
			accepted := false
			if requireLeaseList[maxLen-1] <= maximumLeaseList[maxLen-1] {
				resourceNumList := []int{totalResourceNum}
				if len(maximumLeaseList) > 0 {
					maxInLease := max(maximumLeaseList)
					for j := 0; j < maxInLease; j++ {
						resourceNumList = append(resourceNumList, totalResourceNum)
					}

				}

				feasible, _ = lp.solver.BatchFastCheckIfPackable(requireResourceList,
					requireLeaseList,
					maximumLeaseList,
					globalCacheSolution,
					resourceNumList,
				)
			}
			if !feasible {
				accepted = lp.ProcessUnacceptedJobs(job, requireResourceList, requireLeaseList, maximumLeaseList, globalCacheSolution)
			}
			if feasible || accepted {
				jAttr.ddlTime = leaseTerm * (maximumLeaseList[maxLen-1] - curLeaseIndex)
			}
		} else {
			requireResourceList = requireResourceList[:maxLen-1]
			requireLeaseList = requireLeaseList[:maxLen-1]
			maximumLeaseList = maximumLeaseList[:maxLen-1]
		}
	}

}
