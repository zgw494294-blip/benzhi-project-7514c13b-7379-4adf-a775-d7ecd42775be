package handoff_readiness_stale_cache_test

import (
	"path/filepath"
	"testing"
	"time"

	"corelog/internal/domain"
	"corelog/internal/repository"
	"corelog/internal/service"
)

func TestHandoffReadinessCacheInvalidatedAfterReview(t *testing.T) {
	repo, err := repository.New(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	state := domain.NewState()
	state.Campaigns["campaign-cache"] = domain.DrillingCampaign{
		ID: "campaign-cache", Name: "缓存复核任务", Status: domain.CampaignActive, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	state.Sampling["sampling-cache"] = domain.SamplingRequest{
		ID: "sampling-cache", CampaignID: "campaign-cache", Status: domain.SamplingApproved,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	state.TestResults["result-cache"] = domain.TestResult{
		ID: "result-cache", SamplingRequestID: "sampling-cache", SampleCode: "S-CACHE",
		Method: "XRF", Measurements: map[string]float64{"SiO2": 68.2}, Unit: "%",
		PerformedBy: "实验员", ReviewStatus: domain.ReviewPending, Version: 1, RecordedAt: now,
	}
	if err := repo.Commit(state); err != nil {
		t.Fatal(err)
	}

	svc := service.NewWithDependencies(repo, func() time.Time { return now.Add(time.Minute) }, func(prefix string) string { return prefix + "-fixed" })
	before, err := svc.HandoffReadiness("sampling-cache")
	if err != nil {
		t.Fatal(err)
	}
	if before.CanIssue || before.Pending != 1 {
		t.Fatalf("复核前准备度异常: %+v", before)
	}
	if _, err := svc.ReviewTestResult(service.ReviewTestResultCommand{
		ResultID: "result-cache", Decision: "approve", Reviewer: "质量复核员",
		ReviewNote: "质控通过", ExpectedVersion: 1,
	}, "review-cache-key"); err != nil {
		t.Fatal(err)
	}

	after, err := svc.HandoffReadiness("sampling-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !after.CanIssue || after.Approved != 1 || after.Pending != 0 || len(after.BlockingResults) != 0 {
		t.Fatalf("复核提交后仍返回旧准备度: %+v", after)
	}
}
