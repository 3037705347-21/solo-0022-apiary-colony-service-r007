package status

import (
	"sort"
	"time"

	"example.com/apiary-colony-service/internal/domain"
	"example.com/apiary-colony-service/internal/store"
)

const InspectionWindow = 14 * 24 * time.Hour

type HiveSummary struct {
	HiveID           string     `json:"hiveId"`
	State            string     `json:"state"`
	LastInspectionAt *time.Time `json:"lastInspectionAt"`
	LatestConcerns   []string   `json:"latestConcerns"`
	QueenID          string     `json:"queenId"`
	QueenMarkColor   string     `json:"queenMarkColor"`
}

type Report struct {
	ApiaryID        string         `json:"apiaryId"`
	Name            string         `json:"name"`
	AsOf            time.Time      `json:"asOf"`
	HiveCount       int            `json:"hiveCount"`
	StateCounts     map[string]int `json:"stateCounts"`
	NeedsInspection []string       `json:"needsInspection"`
	ConcernHives    []string       `json:"concernHives"`
	LayingQueens    int            `json:"layingQueens"`
	Hives           []HiveSummary  `json:"hives"`
}

// BuildReport 在单份快照上完成聚合，保证计数与逐蜂箱摘要来自同一视图。
func BuildReport(snapshot store.Snapshot, apiary domain.Apiary, asOf time.Time) Report {
	asOf = asOf.UTC()
	report := Report{
		ApiaryID:        apiary.ID,
		Name:            apiary.Name,
		AsOf:            asOf,
		NeedsInspection: []string{},
		ConcernHives:    []string{},
		Hives:           []HiveSummary{},
	}
	hives := snapshot.ApiaryHives(apiary.ID)
	report.HiveCount = len(hives)
	report.StateCounts = countHiveStates(hives)
	for _, hive := range hives {
		summary := HiveSummary{
			HiveID:           hive.ID,
			State:            string(hive.State),
			LastInspectionAt: hive.LastInspectionAt,
			LatestConcerns:   []string{},
		}
		if queen, ok := snapshot.LayingQueen(hive.ID); ok {
			summary.QueenID = queen.ID
			summary.QueenMarkColor = queen.MarkColor
			report.LayingQueens++
		}
		if latest, ok := snapshot.LatestInspection(hive.ID); ok {
			observedAt := latest.ObservedAt
			summary.LastInspectionAt = &observedAt
			if len(latest.Concerns) > 0 {
				summary.LatestConcerns = append([]string{}, latest.Concerns...)
				report.ConcernHives = append(report.ConcernHives, hive.ID)
			}
		}
		if NeedsInspection(hive, asOf) {
			report.NeedsInspection = append(report.NeedsInspection, hive.ID)
		}
		report.Hives = append(report.Hives, summary)
	}
	sort.Strings(report.NeedsInspection)
	sort.Strings(report.ConcernHives)
	return report
}

// countHiveStates 统计逐蜂箱明细里的状态分布，保证计数与明细同源。
func countHiveStates(hives []domain.Hive) map[string]int {
	counts := initialStateCounts()
	for _, hive := range hives {
		counts[string(hive.State)]++
	}
	return counts
}

// NeedsInspection 判定蜂箱是否需要巡检：从未巡检，或距 asOf 超过巡检窗口。
func NeedsInspection(hive domain.Hive, asOf time.Time) bool {
	if hive.LastInspectionAt == nil {
		return true
	}
	return asOf.Sub(hive.LastInspectionAt.UTC()) > InspectionWindow
}

func initialStateCounts() map[string]int {
	return map[string]int{
		string(domain.HiveColonized):  0,
		string(domain.HiveProduction): 0,
		string(domain.HiveWintering):  0,
		string(domain.HiveRetired):    0,
	}
}
