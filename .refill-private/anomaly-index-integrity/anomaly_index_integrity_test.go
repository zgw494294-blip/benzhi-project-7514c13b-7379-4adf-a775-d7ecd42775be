package anomalyindexintegrity

import (
	"path/filepath"
	"testing"
	"time"

	"corelog/internal/domain"
	"corelog/internal/repository"
)

func TestCommitRejectsAnomalyIndexMismatch(t *testing.T) {
	repo, err := repository.New(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	campaign, err := domain.NewCampaign("c-1", "任务", "平台", "CGCS2000", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	interval, err := domain.NewInterval("i-1", campaign.ID, "ZK01", 0, 10, "花岗岩", 90, "完整", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interval.AddAnomaly("a-1", "破碎", "描述", "照片", "编录员", now); err != nil {
		t.Fatal(err)
	}
	interval.AnomalyIDs = nil
	state := domain.NewState()
	state.Campaigns[campaign.ID] = campaign
	state.Intervals[interval.ID] = interval
	if err := repo.Commit(state); err == nil {
		t.Fatal("异常索引与实体不一致的账本被成功提交")
	}
}
