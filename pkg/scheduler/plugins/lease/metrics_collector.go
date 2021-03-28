package lease

import (
	"fmt"
	"pkg.yezhisheng.me/volcano/pkg/apis/scheduling"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/api"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/framework"
	"pkg.yezhisheng.me/volcano/pkg/scheduler/plugins/util"
	"sync"
	"time"
)

type jobStatistics struct {
	TotalExecutedTime       time.Duration
	TotalLifeTime           time.Duration
	LeaseExpiryCnt          int32
	ExecutedTimeInLastLease time.Duration
	CheckpointCnt           int32
	ColdstartCnt            int32
	Utilized                float64
	Deserved                float64
}
type userStatistics struct {
	utilized        float64
	utilizedTemp    float64
	utilizedHistory []float64
	deserved        float64
	deservedTemp    float64
	deservedHistory []float64
	weighted        float64
	weightedTemp    float64
	weightedHistory []float64
	demand          float64
}

// MetricsCollector defines metrics collector used in lease plugin
type MetricsCollector struct {
	period   time.Duration
	jobPool  map[api.JobID]*jobStatistics
	userPool map[api.QueueID]*userStatistics

	// for active jobs
	userJob map[api.QueueID]map[api.JobID]struct{}
	lock    sync.Mutex

	clusterTotalGPU float64

	ssn *framework.Session

	// n * checkInterval
	userFairnessResetInterval int
	currentInterval           int
	longTermIntervalCnt       int

	// write back results
	lp *leasePlugin
}

// RunOnce should be called in onSessionOpen per session
func (mc *MetricsCollector) RunOnce() {
	mc.worker()
}

// NewMetricsCollector returns a MetricsCollector, should be called only once
func NewMetricsCollector(_period time.Duration, _ssn *framework.Session, _lp *leasePlugin) *MetricsCollector {
	return &MetricsCollector{
		period:   _period,
		lock:     sync.Mutex{},
		jobPool:  make(map[api.JobID]*jobStatistics),
		userPool: make(map[api.QueueID]*userStatistics),
		userJob:  make(map[api.QueueID]map[api.JobID]struct{}),

		ssn: _ssn,
		lp:  _lp,

		// 3600 second
		userFairnessResetInterval: 360,
		currentInterval:           0,
		longTermIntervalCnt:       24,
	}
}

func (mc *MetricsCollector) routineUpdateJobStatistics(jobID api.JobID, stat *jobStatistics, jobNum int, wg *sync.WaitGroup) {
	defer wg.Done()
	stat.TotalLifeTime += mc.period

	job := mc.ssn.Jobs[jobID]

	// share = weighted * total
	share := 1.0 / float64(jobNum) * mc.clusterTotalGPU
	var min float64
	if share < getJobGPUReq(job) {
		min = share
	} else {
		min = getJobGPUReq(job)
	}
	stat.Deserved += min * mc.period.Seconds()

	if job.PodGroup.Status.Phase == scheduling.PodGroupRunning {
		stat.TotalExecutedTime += mc.period
		stat.ExecutedTimeInLastLease += mc.period

		stat.Utilized += getJobGPUReq(job) * mc.period.Seconds()
	}
}

func (mc *MetricsCollector) routineUpdateUserStatistics(user api.QueueID, wg *sync.WaitGroup) {
	defer wg.Done()
	var weighted, utilized, demand float64
	var deserved float64

	quotaRatio := mc.ssn.Queues[user].QuotaRatio
	weighted = quotaRatio * mc.period.Seconds()

	for jobID := range mc.userJob[user] {
		job := mc.ssn.Jobs[jobID]
		if job.PodGroup.Status.Phase == scheduling.PodGroupRunning {
			jobGPUTime := getJobGPUReq(job) * mc.period.Seconds()
			utilized += jobGPUTime
			demand += jobGPUTime
		} else if job.PodGroup.Status.Phase == scheduling.PodGroupPending {
			demand += getJobGPUReq(job) * mc.period.Seconds()
		}
	}

	// deserved = min(demand, weighted)
	if demand < weighted {
		deserved = demand
	} else {
		deserved = weighted
	}

	// update temp
	mc.userPool[user].weightedTemp += weighted
	mc.userPool[user].utilizedTemp += utilized
	mc.userPool[user].deservedTemp += deserved

	// calculate true metrics
	mc.userPool[user].utilized = util.SumSuffix(mc.userPool[user].utilizedHistory, mc.longTermIntervalCnt)
	mc.userPool[user].weighted = util.SumSuffix(mc.userPool[user].weightedHistory, mc.longTermIntervalCnt)
	mc.userPool[user].deserved = util.SumSuffix(mc.userPool[user].deservedHistory, mc.longTermIntervalCnt)
}

func (mc *MetricsCollector) resetUserFairness() {
	// append to history
	for user := range mc.lp.queueOpts {
		mc.userPool[user].deservedHistory = append(mc.userPool[user].deservedHistory, mc.userPool[user].deservedTemp)
		mc.userPool[user].utilizedHistory = append(mc.userPool[user].utilizedHistory, mc.userPool[user].utilizedTemp)
		mc.userPool[user].weightedHistory = append(mc.userPool[user].weightedHistory, mc.userPool[user].weightedTemp)
		mc.userPool[user].deservedTemp = 0
		mc.userPool[user].utilizedTemp = 0
		mc.userPool[user].weightedTemp = 0
	}
}

func (mc *MetricsCollector) worker() {
	// check interval for user fairness reset
	mc.currentInterval++
	if mc.currentInterval == mc.userFairnessResetInterval {
		mc.currentInterval = 0
		mc.resetUserFairness()
	}

	// update metrics according to the most updated jobs and queues info from session
	mc.addIfNotExist()

	activeJobCnt := mc.getActiveJobCnt()

	wg := sync.WaitGroup{}
	for job, stat := range mc.jobPool {
		wg.Add(1)
		// TODO: use activeJobCnt as pending+running, pay attention to sub-job condition
		go mc.routineUpdateJobStatistics(job, stat, activeJobCnt, &wg)
	}
	wg.Wait()

	wg = sync.WaitGroup{}
	for user := range mc.userPool {
		wg.Add(1)
		go mc.routineUpdateUserStatistics(user, &wg)
	}
	wg.Wait()
}

func (mc *MetricsCollector) addIfNotExist() {
	// add queue into userPool
	for queueID := range mc.ssn.Queues {
		if _, exist := mc.userPool[queueID]; !exist {
			mc.userPool[queueID] = initUserStatistics()
			mc.userJob[queueID] = make(map[api.JobID]struct{})
		}
	}
	// add job into jobPool and userJob
	for jobID, job := range mc.ssn.Jobs {
		if _, exist := mc.jobPool[jobID]; !exist {
			mc.jobPool[jobID] = initJobStatistics()
			// userJob should only be inserted when job first met
			mc.userJob[job.Queue][job.UID] = struct{}{}
		}
	}
}

// Delete job from jobPool once finished
func (mc *MetricsCollector) Delete(job api.JobID) {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	// delete from jobPool
	if _, exist := mc.jobPool[job]; exist {
		delete(mc.jobPool, job)
	}
	// delete from userJob map
	queue := mc.getJobQueue(job)
	if _, exist := mc.userJob[queue]; exist {
		if _, exist := mc.userJob[queue][job]; exist {
			delete(mc.userJob[queue], job)
		}
		if len(mc.userJob[queue]) == 0 {
			delete(mc.userJob, queue)
		}
	}
}

func initJobStatistics() *jobStatistics {
	return &jobStatistics{
		TotalExecutedTime:       0,
		TotalLifeTime:           0,
		LeaseExpiryCnt:          0,
		ExecutedTimeInLastLease: 0,
		CheckpointCnt:           0,
		ColdstartCnt:            0,
		Utilized:                0,
		Deserved:                0,
	}
}

func initUserStatistics() *userStatistics {
	return &userStatistics{
		utilized:        0,
		utilizedHistory: nil,
		deserved:        0,
		deservedHistory: nil,
		weighted:        0,
		weightedHistory: nil,
		demand:          0,
	}
}

func (mc *MetricsCollector) AddJobLeaseExpiryCnt(job api.JobID) {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	if stat, exist := mc.jobPool[job]; exist {
		stat.LeaseExpiryCnt++
	}
}

func (mc *MetricsCollector) AddJobCheckpointCnt(job api.JobID) {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	if stat, exist := mc.jobPool[job]; exist {
		stat.CheckpointCnt++
	}
}

func (mc *MetricsCollector) AddJobColdstartCnt(job api.JobID) {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	if stat, exist := mc.jobPool[job]; exist {
		stat.ColdstartCnt++
	}
}

func (mc *MetricsCollector) SetClusterSize(_clusterTotalGPU float64) {
	mc.clusterTotalGPU = _clusterTotalGPU
}

func (mc *MetricsCollector) GetJobStatistics(job api.JobID) (*jobStatistics, error) {
	if _, exist := mc.jobPool[job]; exist {
		return mc.jobPool[job], nil
	}
	return nil, fmt.Errorf("not found")
}

func (mc *MetricsCollector) GetUserStatistics(queue api.QueueID) (*userStatistics, error) {
	if _, exist := mc.userPool[queue]; exist {
		return mc.userPool[queue], nil
	}
	return nil, fmt.Errorf("not found")
}

func (mc *MetricsCollector) getJobQueue(job api.JobID) api.QueueID {
	return mc.ssn.Jobs[job].Queue
}

func (mc *MetricsCollector) getActiveJobCnt() (activeJobCnt int) {
	for user := range mc.userJob {
		activeJobCnt += len(mc.userJob[user])
	}
	return activeJobCnt
}
