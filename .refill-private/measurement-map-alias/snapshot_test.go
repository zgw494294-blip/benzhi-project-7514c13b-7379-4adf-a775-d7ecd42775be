package repository_test

import (
	"path/filepath"
	"testing"
	"time"

	"corelog/internal/domain"
	"corelog/internal/repository"
)

func TestSnapshotCloneKeepsMeasurementsIsolated(t *testing.T) {
	repo, err := repository.New(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	campaign, err := domain.NewCampaign("campaign-1", "北区钻探", "北区平台", "CGCS2000", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	interval, err := domain.NewInterval("interval-1", campaign.ID, "ZK01", 0, 10, "花岗岩", 95, "完整", now)
	if err != nil {
		t.Fatal(err)
	}
	request, err := domain.NewSamplingRequest("sampling-1", campaign.ID, []string{interval.ID}, map[string]int64{interval.ID: interval.Version}, "主量元素检测", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := domain.NewTestResult("result-1", request.ID, "ZK01-S001", "XRF", map[string]float64{"SiO2": 67.3}, "%", "实验员", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Transact(func(state *domain.State) error {
		state.Campaigns[campaign.ID] = campaign
		state.Intervals[interval.ID] = interval
		state.Sampling[request.ID] = request
		state.TestResults[result.ID] = result
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	snapshot := repo.Snapshot()
	snapshot.TestResults[result.ID].Measurements["SiO2"] = 12.4
	second := repo.Snapshot()
	if got := second.TestResults[result.ID].Measurements["SiO2"]; got != 67.3 {
		t.Fatalf("修改快照污染了账本检测值: got=%v want=67.3", got)
	}
}
