package selfcheck

import (
	"sort"
	"time"

	"corelog/internal/domain"
)

type CheckItem struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type Report struct {
	PassedAt time.Time   `json:"passedAt"`
	Passed   bool        `json:"passed"`
	Checks   []CheckItem `json:"checks"`
	Failures []string    `json:"failures"`
}

func Run(state domain.State, now time.Time) Report {
	report := Report{PassedAt: now.UTC(), Checks: []CheckItem{}, Failures: []string{}}
	checks := []struct {
		name   string
		detail string
		check  func() bool
	}{
		{"campaign_references", "孔段、申请、检测和凭据引用的任务均存在", func() bool {
			for _, interval := range state.Intervals {
				if _, ok := state.Campaigns[interval.CampaignID]; !ok {
					return false
				}
			}
			for _, request := range state.Sampling {
				if _, ok := state.Campaigns[request.CampaignID]; !ok {
					return false
				}
			}
			return true
		}},
		{"interval_continuity", "同一钻孔孔段无重叠且深度连续", func() bool {
			byBorehole := make(map[string][]domain.CoreInterval)
			for _, interval := range state.Intervals {
				key := interval.CampaignID + "\x00" + interval.BoreholeCode
				byBorehole[key] = append(byBorehole[key], interval)
			}
			for _, intervals := range byBorehole {
				sort.Slice(intervals, func(i, j int) bool { return intervals[i].DepthStart < intervals[j].DepthStart })
				for idx := 1; idx < len(intervals); idx++ {
					if intervals[idx].DepthStart < intervals[idx-1].DepthEnd-0.000001 || intervals[idx].DepthStart-intervals[idx-1].DepthEnd > 0.000001 {
						return false
					}
				}
			}
			return true
		}},
		{"anomaly_evidence", "异常证据和处置状态完整一致", func() bool {
			for _, interval := range state.Intervals {
				for _, anomaly := range interval.Anomalies {
					if anomaly.ValidateEvidence() != nil {
						return false
					}
				}
			}
			return true
		}},
		{"sampling_versions", "取样申请冻结的孔段版本仍然一致", func() bool {
			for _, request := range state.Sampling {
				if request.Status == domain.SamplingReturned {
					continue
				}
				for _, intervalID := range request.IntervalIDs {
					interval, ok := state.Intervals[intervalID]
					if !ok || request.IntervalVersions[intervalID] != interval.FrozenVersion || interval.FrozenBy != request.ID {
						return false
					}
				}
			}
			return true
		}},
		{"approved_results", "已批准取样申请的检测结果均完成质量复核", func() bool {
			for _, request := range state.Sampling {
				if request.Status != domain.SamplingApproved && request.Status != domain.SamplingClosed {
					continue
				}
				found := false
				for _, result := range state.TestResults {
					if result.SamplingRequestID == request.ID {
						found = true
						if result.ReviewStatus != domain.ReviewApproved {
							return false
						}
					}
				}
				if !found {
					return false
				}
			}
			return true
		}},
		{"certificate_hashes", "交接凭据校验信息可重复验证", func() bool {
			for _, certificate := range state.Certificates {
				valid, err := domain.VerifyCertificate(certificate)
				if err != nil || !valid {
					return false
				}
			}
			return true
		}},
	}
	for _, item := range checks {
		passed := item.check()
		report.Checks = append(report.Checks, CheckItem{Name: item.name, Passed: passed, Detail: item.detail})
		if !passed {
			report.Failures = append(report.Failures, item.name)
		}
	}
	report.Passed = len(report.Failures) == 0
	return report
}
