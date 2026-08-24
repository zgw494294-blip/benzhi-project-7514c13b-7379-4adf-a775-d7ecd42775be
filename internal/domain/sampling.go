package domain

import (
	"strings"
	"time"
)

func NewSamplingRequest(id, campaignID string, intervalIDs []string, versions map[string]int64, purpose, requestedBy string, now time.Time) (SamplingRequest, error) {
	for field, value := range map[string]string{"id": id, "campaignID": campaignID, "purpose": purpose, "requestedBy": requestedBy} {
		if strings.TrimSpace(value) == "" {
			return SamplingRequest{}, Required(field)
		}
	}
	if len(intervalIDs) == 0 {
		return SamplingRequest{}, Invalid("intervalIDs", "至少选择一个孔段")
	}
	seen := make(map[string]bool, len(intervalIDs))
	for _, intervalID := range intervalIDs {
		if strings.TrimSpace(intervalID) == "" {
			return SamplingRequest{}, Invalid("intervalIDs", "不能包含空标识")
		}
		if seen[intervalID] {
			return SamplingRequest{}, Invalid("intervalIDs", "不能包含重复孔段")
		}
		seen[intervalID] = true
		if versions[intervalID] < 1 {
			return SamplingRequest{}, Invalid("intervalVersions", "必须提供每个孔段的有效版本")
		}
	}
	return SamplingRequest{
		ID: id, CampaignID: campaignID, IntervalIDs: append([]string(nil), intervalIDs...),
		IntervalVersions: cloneVersions(versions), Purpose: strings.TrimSpace(purpose), RequestedBy: strings.TrimSpace(requestedBy),
		Status: SamplingPending, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}, nil
}

func (r *SamplingRequest) CheckVersion(expected int64) error {
	if expected < 1 {
		return Invalid("expectedVersion", "必须大于 0")
	}
	if r.Version != expected {
		return Conflict("取样申请版本已变化")
	}
	return nil
}

func (r *SamplingRequest) Review(approved bool, reviewer, note string, now time.Time) error {
	if r.Status != SamplingPending {
		return StateConflict("仅待复核申请可以提交复核结论")
	}
	if strings.TrimSpace(reviewer) == "" {
		return Required("reviewer")
	}
	if strings.TrimSpace(note) == "" {
		return Required("reviewNote")
	}
	r.Reviewer = strings.TrimSpace(reviewer)
	r.ReviewNote = strings.TrimSpace(note)
	if approved {
		r.Status = SamplingApproved
	} else {
		r.Status = SamplingReturned
	}
	r.Version++
	r.UpdatedAt = now.UTC()
	return nil
}

func (r *SamplingRequest) Resubmit(versions map[string]int64, purpose, requestedBy string, now time.Time) error {
	if r.Status != SamplingReturned {
		return StateConflict("仅退回的取样申请可以补正重提")
	}
	if strings.TrimSpace(purpose) == "" {
		return Required("purpose")
	}
	if strings.TrimSpace(requestedBy) == "" {
		return Required("requestedBy")
	}
	for _, intervalID := range r.IntervalIDs {
		if versions[intervalID] < 1 {
			return Invalid("intervalVersions", "必须提供每个孔段的最新版本")
		}
	}
	r.IntervalVersions = cloneVersions(versions)
	r.Purpose = strings.TrimSpace(purpose)
	r.RequestedBy = strings.TrimSpace(requestedBy)
	r.Status = SamplingPending
	r.Reviewer = ""
	r.ReviewNote = ""
	r.Version++
	r.UpdatedAt = now.UTC()
	return nil
}

func (r *SamplingRequest) Close(now time.Time) error {
	if r.Status != SamplingApproved {
		return StateConflict("只有已批准申请可以签发交接凭据")
	}
	r.Status = SamplingClosed
	r.Version++
	r.UpdatedAt = now.UTC()
	return nil
}

func cloneVersions(source map[string]int64) map[string]int64 {
	copy := make(map[string]int64, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
