package lease

import (
	"math"

	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

func (lp *leasePlugin) FlushSpotGuaranteeResource(ssn *framework.Session) {
	GuaranteeResource = 0

	for _, job := range ssn.Jobs {
		if isFinishedJob(job) {
			continue
		}
		jAttr, found := lp.jobAttrs[job.UID]
		if found {
			if jAttr.occupyLease == 1 {
				GuaranteeResource += jAttr.reqGPU
			} else if jAttr.forceGuarantee == 1 {
				GuaranteeResource += jAttr.reqGPU
			}

		}
	}
	SpotResource = totalResourceNum - GuaranteeResource

}
func (lp *leasePlugin) FlushLeaseJobs(ssn *framework.Session) {
	requireResourceList, requireLeaseList, maximumLeaseList, inBlockList, remainingResourceList, outBlockList := lp.LeaseInfoCollection(ssn)
	if len(requireLeaseList) > 0 {
		_, solutionMatrix := lp.solver.JobSelection(requireResourceList, requireLeaseList, maximumLeaseList, remainingResourceList)
		occupyList := append(inBlockList, outBlockList...)
		for idx, job := range occupyList {
			jAttr := lp.jobAttrs[job.UID]
			jAttr.cacheSolution = solutionMatrix[idx]
			if jAttr.cacheSolution[0] > 0 {
				jAttr.occupyLease = 1
			} else {
				jAttr.occupyLease = 0
			}
			jAttr.cacheSolution = jAttr.cacheSolution[1:]
		}
	}
}

func (lp *leasePlugin) FlushRunningJobs(ssn *framework.Session) {
	for _, job := range ssn.Jobs {
		if isRunningJob(job) {
			jAttr, _ := lp.jobAttrs[job.UID]
			jAttr.remainingTime -= leaseTerm
			jAttr.emergence = computeEmergence(jAttr.remainingTime, jAttr.reqGPU)
			if jAttr.sloPrioirity == acceptedSLO && jAttr.occupyLease == 0 {
				stillRequireLease := int(math.Ceil(float64(jAttr.remainingTime) / float64(leaseTerm)))
				if stillRequireLease != jAttr.stillRequireLease {
					for cache_iter := len(jAttr.cacheSolution) - 1; cache_iter >= 0; cache_iter-- {
						if jAttr.cacheSolution[cache_iter] == 1 {
							jAttr.cacheSolution[cache_iter] = 0
							break
						}
					}
				}
			}
		}
	}
}

func (lp *leasePlugin) FlushPendingJobs(ssn *framework.Session) {
	for _, job := range ssn.Jobs {
		if isPendingJob(job) {
			jAttr, _ := lp.jobAttrs[job.UID]
			jAttr.emergence = computeEmergence(jAttr.remainingTime, jAttr.reqGPU)
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
			duration:               job.Duration,
			submitTime:             job.SubmitTime,
			remainingTime:          int(float64(job.Duration) * 1.2),
			ddlTime:                job.DDLTime,
		}
		jAttr.stillRequireLease = int(math.Ceil(float64(jAttr.remainingTime) / float64(leaseTerm)))

		maximumLease := computeMaximumLease(jAttr.ddlTime+jAttr.submitTime, leaseTerm, curLeaseIndex)
		requireResourceList = append(requireResourceList, jAttr.reqGPU)
		requireLeaseList = append(requireLeaseList, jAttr.stillRequireLease)
		maximumLeaseList = append(maximumLeaseList, maximumLease)
		if eventGuaranteeResource <= SpotResource {
			if jAttr.remainingTime <= inLeaseRemainingTime {
				jAttr.forceGuarantee = 1
				eventGuaranteeResource += jAttr.reqGPU
				jAttr.requireMipOptimization = 0
			}
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
				jAttr.ddlTime = leaseTerm*(maximumLeaseList[maxLen-1]+curLeaseIndex) - jAttr.submitTime
			}
		} else {
			requireResourceList = requireResourceList[:maxLen-1]
			requireLeaseList = requireLeaseList[:maxLen-1]
			maximumLeaseList = maximumLeaseList[:maxLen-1]
		}
	}

}
