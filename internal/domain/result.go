package domain

import (
	"math"
	"strings"
	"time"
)

func NewTestResult(id, requestID, sampleCode, method string, measurements map[string]float64, unit, performedBy string, now time.Time) (TestResult, error) {
	for field, value := range map[string]string{"id": id, "samplingRequestID": requestID, "sampleCode": sampleCode, "method": method, "unit": unit, "performedBy": performedBy} {
		if strings.TrimSpace(value) == "" {
			return TestResult{}, Required(field)
		}
	}
	if len(measurements) == 0 {
		return TestResult{}, Invalid("measurements", "至少包含一个检测指标")
	}
	copyMeasurements := make(map[string]float64, len(measurements))
	for name, value := range measurements {
		if strings.TrimSpace(name) == "" {
			return TestResult{}, Invalid("measurements", "指标名称不能为空")
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return TestResult{}, Invalid("measurements", "指标值必须是有限数值")
		}
		copyMeasurements[strings.TrimSpace(name)] = value
	}
	return TestResult{
		ID: id, SamplingRequestID: requestID, SampleCode: strings.TrimSpace(sampleCode), Method: strings.TrimSpace(method),
		Measurements: copyMeasurements, Unit: strings.TrimSpace(unit), PerformedBy: strings.TrimSpace(performedBy),
		ReviewStatus: ReviewPending, Version: 1, RecordedAt: now.UTC(),
	}, nil
}

func (r *TestResult) CheckVersion(expected int64) error {
	if expected < 1 {
		return Invalid("expectedVersion", "必须大于 0")
	}
	if r.Version != expected {
		return Conflict("检测结果版本已变化")
	}
	return nil
}

func (r *TestResult) Review(approved bool, reviewer, note string, now time.Time) error {
	if r.ReviewStatus != ReviewPending && r.ReviewStatus != ReviewReturned {
		return StateConflict("该检测结果已经通过质量复核")
	}
	if strings.TrimSpace(reviewer) == "" {
		return Required("reviewer")
	}
	if strings.TrimSpace(note) == "" {
		return Required("reviewNote")
	}
	if approved {
		r.ReviewStatus = ReviewApproved
	} else {
		r.ReviewStatus = ReviewReturned
	}
	r.ReviewedBy = strings.TrimSpace(reviewer)
	r.ReviewNote = strings.TrimSpace(note)
	reviewedAt := now.UTC()
	r.ReviewedAt = &reviewedAt
	r.Version++
	return nil
}

func CalculateHandoffReadiness(requestID string, results []TestResult) HandoffReadiness {
	readiness := HandoffReadiness{SamplingRequestID: requestID, BlockingResults: []string{}}
	for _, result := range results {
		readiness.Total++
		switch result.ReviewStatus {
		case ReviewPending:
			readiness.Pending++
			readiness.BlockingResults = append(readiness.BlockingResults, result.ID)
		case ReviewReturned:
			readiness.Returned++
			readiness.BlockingResults = append(readiness.BlockingResults, result.ID)
		case ReviewApproved:
			readiness.Approved++
		default:
			readiness.BlockingResults = append(readiness.BlockingResults, result.ID)
		}
	}
	readiness.CanIssue = readiness.Total > 0 && readiness.Approved == readiness.Total
	return readiness
}
