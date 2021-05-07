package lease

import (
	"k8s.io/klog"
	"strconv"
)

const (
	podGroupStatisticsGroupAnnoKey = "scheduling.yzs981130.io/jobstatistics/"
	userStatisticsGroupAnnoKey = "scheduling.yzs981130.io/userstatistics/"
)

var PodGroupStatisticsAnnoKey = struct {
	TotalExecutedTime string
	TotalLifeTime string
	LeaseExpiryCnt	string
	ExecutedTimeInLastLease string
	CheckpointCnt	string
	ColdstartCnt	string
	Utilized		string
	Deserved		string
}{
	podGroupStatisticsGroupAnnoKey + "TotalExecutedTime",
	podGroupStatisticsGroupAnnoKey + "TotalLifeTime",
	podGroupStatisticsGroupAnnoKey + "LeaseExpiryCnt",
	podGroupStatisticsGroupAnnoKey + "ExecutedTimeInLastLease",
	podGroupStatisticsGroupAnnoKey + "CheckpointCnt",
	podGroupStatisticsGroupAnnoKey + "ColdstartCnt",
	podGroupStatisticsGroupAnnoKey + "Utilized",
	podGroupStatisticsGroupAnnoKey + "Deserved",
}

var UserStatisticsAnnoKey = struct {
	Utilized	string
	Deserved	string
	Weighted 	string
	UdivideW	string
}{
	userStatisticsGroupAnnoKey + "Utilized",
	userStatisticsGroupAnnoKey + "Deserved",
	userStatisticsGroupAnnoKey + "Weighted",
	userStatisticsGroupAnnoKey + "UdivideW",
}

func string2float(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		klog.Errorf("error in string2float for string %s", s)
	}
	return f
}
