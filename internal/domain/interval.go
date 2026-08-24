package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const depthTolerance = 0.000001

func NewInterval(id, campaignID, boreholeCode string, start, end float64, lithology string, recovery float64, condition string, now time.Time) (CoreInterval, error) {
	for field, value := range map[string]string{"id": id, "campaignID": campaignID, "boreholeCode": boreholeCode, "lithology": lithology, "condition": condition} {
		if strings.TrimSpace(value) == "" {
			return CoreInterval{}, Required(field)
		}
	}
	if math.IsNaN(start) || math.IsInf(start, 0) || start < 0 {
		return CoreInterval{}, Invalid("depthStart", "必须是非负有限数值")
	}
	if math.IsNaN(end) || math.IsInf(end, 0) || end <= start {
		return CoreInterval{}, Invalid("depthEnd", "必须大于起始深度")
	}
	if math.IsNaN(recovery) || math.IsInf(recovery, 0) || recovery < 0 || recovery > 100 {
		return CoreInterval{}, Invalid("recoveryRate", "必须在 0 到 100 之间")
	}
	return CoreInterval{
		ID: strings.TrimSpace(id), CampaignID: strings.TrimSpace(campaignID), BoreholeCode: strings.TrimSpace(boreholeCode),
		DepthStart: start, DepthEnd: end, Lithology: strings.TrimSpace(lithology), RecoveryRate: recovery,
		Condition: strings.TrimSpace(condition), AnomalyIDs: []string{}, Anomalies: []Anomaly{}, Version: 1,
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}, nil
}

func ValidateIntervalPlacement(candidate CoreInterval, existing []CoreInterval) error {
	var same []CoreInterval
	for _, interval := range existing {
		if interval.CampaignID == candidate.CampaignID && interval.BoreholeCode == candidate.BoreholeCode {
			same = append(same, interval)
		}
	}
	same = append(same, candidate)
	sort.Slice(same, func(i, j int) bool { return same[i].DepthStart < same[j].DepthStart })
	for i := 1; i < len(same); i++ {
		previous, current := same[i-1], same[i]
		if current.DepthStart < previous.DepthEnd-depthTolerance {
			return Conflict("同一钻孔的孔段深度发生重叠")
		}
		if math.Abs(current.DepthStart-previous.DepthEnd) > depthTolerance {
			return Conflict("同一钻孔的孔段必须连续编录")
		}
	}
	return nil
}

// ValidateIntervalBatch checks a complete batch against existing ledger entries.
// Candidates are indexed in the same order as the request so callers can locate errors.
func ValidateIntervalBatch(candidates []CoreInterval, existing []CoreInterval) error {
	if len(candidates) == 0 {
		return Invalid("intervals", "批量孔段不能为空")
	}
	all := append([]CoreInterval(nil), existing...)
	for idx, candidate := range candidates {
		if err := ValidateIntervalPlacement(candidate, all); err != nil {
			if e, ok := err.(*Error); ok {
				return NewError(e.Code, e.Message, fmt.Sprintf("intervals[%d]", idx))
			}
			return err
		}
		all = append(all, candidate)
	}
	return nil
}

func SummarizeIntervalProgress(intervals []CoreInterval) []BoreholeProgress {
	byBorehole := make(map[string][]CoreInterval)
	for _, interval := range intervals {
		byBorehole[interval.BoreholeCode] = append(byBorehole[interval.BoreholeCode], interval)
	}
	result := make([]BoreholeProgress, 0, len(byBorehole))
	for borehole, items := range byBorehole {
		sort.Slice(items, func(i, j int) bool { return items[i].DepthStart < items[j].DepthStart })
		progress := BoreholeProgress{BoreholeCode: borehole, IntervalCount: len(items), DepthStart: items[0].DepthStart, DepthEnd: items[len(items)-1].DepthEnd, ContinuityStatus: "continuous"}
		for idx := 1; idx < len(items); idx++ {
			if items[idx].DepthStart < items[idx-1].DepthEnd-depthTolerance || math.Abs(items[idx].DepthStart-items[idx-1].DepthEnd) > depthTolerance {
				progress.ContinuityStatus = "discontinuous"
				break
			}
		}
		latest := items[0]
		for _, item := range items[1:] {
			if item.Version > latest.Version || (item.Version == latest.Version && (item.UpdatedAt.After(latest.UpdatedAt) || (item.UpdatedAt.Equal(latest.UpdatedAt) && item.DepthEnd > latest.DepthEnd))) {
				latest = item
			}
		}
		progress.LatestIntervalID, progress.LatestVersion = latest.ID, latest.Version
		result = append(result, progress)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].BoreholeCode < result[j].BoreholeCode })
	return result
}

func (a Anomaly) ValidateEvidence() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.Kind) == "" || strings.TrimSpace(a.Evidence) == "" || strings.TrimSpace(a.ReportedBy) == "" {
		return Invalid("anomaly", "异常记录缺少证据或报告信息")
	}
	if a.Resolved {
		if strings.TrimSpace(a.Resolution) == "" || strings.TrimSpace(a.ResolvedBy) == "" || a.ResolvedAt == nil {
			return Invalid("anomaly", "已处置异常缺少处置记录")
		}
	} else if strings.TrimSpace(a.Resolution) != "" || strings.TrimSpace(a.ResolvedBy) != "" || a.ResolvedAt != nil {
		return Invalid("anomaly", "未处置异常包含处置记录")
	}
	return nil
}

func (i *CoreInterval) CheckVersion(expected int64) error {
	if expected < 1 {
		return Invalid("expectedVersion", "必须大于 0")
	}
	if i.Version != expected {
		return Conflict("孔段版本已变化")
	}
	return nil
}

func (i *CoreInterval) EnsureMutable() error {
	if i.FrozenBy != "" {
		return StateConflict("孔段已被取样申请冻结")
	}
	return nil
}

func (i *CoreInterval) AddAnomaly(id, kind, description, evidence, reportedBy string, now time.Time) (Anomaly, error) {
	if err := i.EnsureMutable(); err != nil {
		return Anomaly{}, err
	}
	for field, value := range map[string]string{"id": id, "kind": kind, "description": description, "evidence": evidence, "reportedBy": reportedBy} {
		if strings.TrimSpace(value) == "" {
			return Anomaly{}, Required(field)
		}
	}
	anomaly := Anomaly{ID: id, Kind: strings.TrimSpace(kind), Description: strings.TrimSpace(description), Evidence: strings.TrimSpace(evidence), ReportedBy: strings.TrimSpace(reportedBy), ReportedAt: now.UTC()}
	i.Anomalies = append(i.Anomalies, anomaly)
	i.AnomalyIDs = append(i.AnomalyIDs, id)
	i.Version++
	i.UpdatedAt = now.UTC()
	return anomaly, nil
}

func (i *CoreInterval) ResolveAnomaly(anomalyID, resolution, resolvedBy string, now time.Time) (Anomaly, error) {
	if err := i.EnsureMutable(); err != nil {
		return Anomaly{}, err
	}
	if strings.TrimSpace(resolution) == "" {
		return Anomaly{}, Required("resolution")
	}
	if strings.TrimSpace(resolvedBy) == "" {
		return Anomaly{}, Required("resolvedBy")
	}
	for idx := range i.Anomalies {
		if i.Anomalies[idx].ID != anomalyID {
			continue
		}
		if i.Anomalies[idx].Resolved {
			return Anomaly{}, StateConflict("异常已完成处置")
		}
		i.Anomalies[idx].Resolved = true
		i.Anomalies[idx].Resolution = strings.TrimSpace(resolution)
		i.Anomalies[idx].ResolvedBy = strings.TrimSpace(resolvedBy)
		resolvedAt := now.UTC()
		i.Anomalies[idx].ResolvedAt = &resolvedAt
		i.Version++
		i.UpdatedAt = now.UTC()
		return i.Anomalies[idx], nil
	}
	return Anomaly{}, NotFound("异常标记")
}

func (i CoreInterval) HasOpenAnomalies() bool {
	for _, anomaly := range i.Anomalies {
		if !anomaly.Resolved {
			return true
		}
	}
	return false
}

func (i *CoreInterval) Freeze(requestID string) error {
	if i.FrozenBy != "" && i.FrozenBy != requestID {
		return StateConflict("孔段已被其他取样申请冻结")
	}
	i.FrozenBy = requestID
	i.FrozenVersion = i.Version
	return nil
}

func (i *CoreInterval) Unfreeze(requestID string) {
	if i.FrozenBy == requestID {
		i.FrozenBy = ""
		i.FrozenVersion = 0
	}
}
