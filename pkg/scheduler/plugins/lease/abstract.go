package lease

import (
	"sort"

	"k8s.io/klog"
	"volcano.sh/volcano/pkg/scheduler/api"
	"volcano.sh/volcano/pkg/scheduler/framework"
)

const MAX_SEARCH_JOB = 200

type LeaseInfoItem struct {
	a int
	b int
	c int
	d *api.JobInfo
}

type LeaseInfoLister []LeaseInfoItem

func (s LeaseInfoLister) Len() int      { return len(s) }
func (s LeaseInfoLister) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s LeaseInfoLister) Less(i, j int) bool {
	if s[i].a != s[j].a {
		return s[i].a > s[j].a
	}
	return s[i].c > s[j].c
}

func (lp *leasePlugin) AbstractLeaseList(ssn *framework.Session) ([]int, []int, []int, []*api.JobInfo) {
	requireResourceList := []int{}
	requireLeaseList := []int{}
	maximumLeaseList := []int{}
	inBlockList := []*api.JobInfo{}

	for _, job := range ssn.Jobs {
		if !isFinishedJob(job) {
			jAttr := lp.jobAttrs[job.UID]
			if jAttr.requireMipOptimization == 0 {
				continue
			}
			requireResource := jAttr.reqGPU
			requireLease := jAttr.stillRequireLease
			maximumLease := computeMaximumLease(jAttr.ddlTime+jAttr.submitTime, leaseTerm, curLeaseIndex)
			// do something tricky
			if maximumLease < requireLease {
				klog.V(4).Infof("lease, job %s/%d maximuLease/requireLease %d%d", job.Namespace, job.UID, maximumLease, requireLease)
			}
			if requireLease > 0 {
				requireResourceList = append(requireResourceList, requireResource)
				requireLeaseList = append(requireLeaseList, requireLease)
				maximumLeaseList = append(maximumLeaseList, maximumLease)
				inBlockList = append(inBlockList, job)
			}
		}
	}
	return requireResourceList, requireLeaseList, maximumLeaseList, inBlockList
}

func (lp *leasePlugin) AbstractLeaseWithCacheList(jobList []*api.JobInfo) ([]int, []int, []int, []*api.JobInfo, []int) {
	// int
	requireResourceList := []int{}
	requireLeaseList := []int{}
	maximumLeaseList := []int{}
	inBlockList := []*api.JobInfo{}

	no_cache_required_resource_list := []int{}
	no_cache_required_lease_list := []int{}
	no_cache_maximum_lease_list := []int{}
	no_cache_in_block_list := []*api.JobInfo{}

	for _, job := range jobList {
		if !isFinishedJob(job) {
			jAttr := lp.jobAttrs[job.UID]
			if jAttr.requireMipOptimization == 0 {
				continue
			}
			requireResource := jAttr.reqGPU
			requireLease := jAttr.stillRequireLease
			maximumLease := computeMaximumLease(jAttr.ddlTime+jAttr.submitTime, leaseTerm, curLeaseIndex)
			// do something tricky
			if maximumLease < requireLease {
				klog.V(4).Infof("lease, job %s/%d maximuLease/requireLease %d%d", job.Namespace, job.UID, maximumLease, requireLease)
			}
			if requireLease > 0 {
				requireResourceList = append(requireResourceList, requireResource)
				requireLeaseList = append(requireLeaseList, requireLease)
				maximumLeaseList = append(maximumLeaseList, maximumLease)
				inBlockList = append(inBlockList, job)
			}
		}
	}
	cacheSolution := []int{}
	if len(maximumLeaseList) > 0 {
		maxLen := max(maximumLeaseList)
		for i := 0; i < maxLen; i++ {
			cacheSolution = append(cacheSolution, totalResourceNum)
		}
	}
	for i, job := range inBlockList {
		jAttr := lp.jobAttrs[job.UID]
		requireResource := requireResourceList[i]
		requireLease := requireLeaseList[i]
		maximumLease := maximumLeaseList[i]
		if jAttr.cacheSolution == nil {
			no_cache_required_resource_list = append(no_cache_required_resource_list, requireResource)
			no_cache_required_lease_list = append(no_cache_required_lease_list, requireLease)
			no_cache_maximum_lease_list = append(no_cache_maximum_lease_list, maximumLease)
			no_cache_in_block_list = append(no_cache_in_block_list, job)
		} else {
			if len(jAttr.cacheSolution) != maximumLease {
				klog.V(4).Infof("cache solution mismatch with maximumLease, job %s/%d maximuLease/requireLease %d%d", job.Namespace, job.UID, maximumLease, requireLease)
			}
			for idx, occupy := range jAttr.cacheSolution {
				cacheSolution[idx] -= int(occupy * jAttr.reqGPU)
				if cacheSolution[idx] < 0 {
					klog.V(4).Infof("cache solution < 0, job %s/%d maximuLease/requireLease %d%d", job.Namespace, job.UID, maximumLease, requireLease)
				}
			}
		}

	}
	return no_cache_required_resource_list, no_cache_required_lease_list, no_cache_maximum_lease_list, no_cache_in_block_list, cacheSolution
}

func (lp *leasePlugin) LeaseInfoCollection(ssn *framework.Session) ([]int, []int, []int, []*api.JobInfo, []int, []*api.JobInfo) {
	requireResourceList, requireLeaseList, maximumLeaseList, inBlockList := lp.AbstractLeaseList(ssn)
	maxLen := 0
	if len(maximumLeaseList) == 0 {
		maxLen = 1
	} else {
		maxLen = max(maximumLeaseList)
	}
	if maxLen < 1 {
		maxLen = 1
	}
	remainingResourceList := []int{}
	for i := 0; i < maxLen; i++ {
		remainingResourceList = append(remainingResourceList, totalResourceNum)
	}

	outBlockList := []*api.JobInfo{}
	if len(requireLeaseList) > MAX_SEARCH_JOB {
		leaseInfoList := LeaseInfoLister{}
		for i := 0; i < len(requireLeaseList); i++ {
			requireResource := requireResourceList[i]
			requireLease := requireLeaseList[i]
			maximuLease := maximumLeaseList[i]
			inBlockJob := inBlockList[i]
			leaseInfoItem := LeaseInfoItem{requireResource, requireLease, maximuLease, inBlockJob}
			leaseInfoList = append(leaseInfoList, leaseInfoItem)
		}
		sort.Sort(leaseInfoList)
		remove_count := len(leaseInfoList) - MAX_SEARCH_JOB

		// init again
		requireResourceList := []int{}
		requireLeaseList := []int{}
		maximumLeaseList := []int{}
		inBlockList := []*api.JobInfo{}
		for _, leaseInfo := range leaseInfoList {
			requireResource := leaseInfo.a
			requireLease := leaseInfo.b
			maximuLease := leaseInfo.c
			job := leaseInfo.d
			jAttr := lp.jobAttrs[job.UID]

			if jAttr.cacheSolution == nil && (remove_count > 0) {
				outBlockList = append(outBlockList, job)
				for j, occupy := range jAttr.cacheSolution {
					remainingResourceList[j] -= int(occupy * jAttr.reqGPU)
				}
				remove_count -= 1
			} else {
				requireResourceList = append(requireResourceList, requireResource)
				requireLeaseList = append(requireLeaseList, requireLease)
				maximumLeaseList = append(maximumLeaseList, maximuLease)
				inBlockList = append(inBlockList, job)
			}
		}
	}
	return requireResourceList, requireLeaseList, maximumLeaseList, inBlockList, remainingResourceList, outBlockList
}
