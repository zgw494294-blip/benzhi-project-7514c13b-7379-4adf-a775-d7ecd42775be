package snapshotpointeralias

import (
	"path/filepath"
	"testing"
	"time"

	"corelog/internal/domain"
	"corelog/internal/repository"
)

func TestSnapshotCloneKeepsResolvedAtIsolated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	repo, err := repository.New(path)
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
	if _, err := interval.ResolveAnomaly("a-1", "已复核", "复核员", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	state := domain.NewState()
	state.Campaigns[campaign.ID] = campaign
	state.Intervals[interval.ID] = interval
	if err := repo.Commit(state); err != nil {
		t.Fatal(err)
	}

	snapshot := repo.Snapshot()
	changed := now.Add(72 * time.Hour)
	*snapshot.Intervals[interval.ID].Anomalies[0].ResolvedAt = changed
	stored := repo.Snapshot().Intervals[interval.ID].Anomalies[0].ResolvedAt
	if stored == nil || !stored.Equal(now.Add(time.Hour)) {
		t.Fatalf("快照修改污染了账本中的处置时间: got %v", stored)
	}
}
