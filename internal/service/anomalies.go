package service

import (
	"sort"
	"strings"

	"corelog/internal/domain"
)

func (s *Service) ListAnomalies(campaignID, status, kind string) (domain.AnomalySummary, error) {
	state := s.repo.Snapshot()
	if _, ok := state.Campaigns[campaignID]; !ok {
		return domain.AnomalySummary{}, domain.NotFound("钻探任务")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "open" && status != "resolved" {
		return domain.AnomalySummary{}, domain.Invalid("status", "必须为 open、resolved 或 all")
	}
	kind = strings.TrimSpace(kind)
	result := domain.AnomalySummary{Items: []domain.AnomalyEvidence{}, BlockingBoreholes: []string{}, BlockingIntervals: []string{}}
	blocking := make(map[string]bool)
	blockingIntervals := make(map[string]bool)
	for _, interval := range state.Intervals {
		if interval.CampaignID != campaignID {
			continue
		}
		for _, anomaly := range interval.Anomalies {
			if err := anomaly.ValidateEvidence(); err != nil {
				return domain.AnomalySummary{}, err
			}
			if kind != "" && anomaly.Kind != kind {
				continue
			}
			isOpen := !anomaly.Resolved
			if status == "open" && !isOpen || status == "resolved" && isOpen {
				continue
			}
			item := domain.AnomalyEvidence{ID: anomaly.ID, BoreholeCode: interval.BoreholeCode, IntervalID: interval.ID, DepthStart: interval.DepthStart, DepthEnd: interval.DepthEnd, Kind: anomaly.Kind, Description: anomaly.Description, Evidence: anomaly.Evidence, ReportedBy: anomaly.ReportedBy, ReportedAt: anomaly.ReportedAt, Status: "resolved"}
			if isOpen {
				item.Status = "open"
				blocking[interval.BoreholeCode] = true
				blockingIntervals[interval.ID] = true
			}
			result.Items = append(result.Items, item)
		}
	}
	result.Total = len(result.Items)
	for _, item := range result.Items {
		if item.Status == "open" {
			result.Open++
		} else {
			result.Resolved++
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].BoreholeCode != result.Items[j].BoreholeCode {
			return result.Items[i].BoreholeCode < result.Items[j].BoreholeCode
		}
		if result.Items[i].DepthStart != result.Items[j].DepthStart {
			return result.Items[i].DepthStart < result.Items[j].DepthStart
		}
		return result.Items[i].ID < result.Items[j].ID
	})
	for borehole := range blocking {
		result.BlockingBoreholes = append(result.BlockingBoreholes, borehole)
	}
	sort.Strings(result.BlockingBoreholes)
	for intervalID := range blockingIntervals {
		result.BlockingIntervals = append(result.BlockingIntervals, intervalID)
	}
	sort.Strings(result.BlockingIntervals)
	return result, nil
}
