package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"corelog/internal/domain"
)

func TestCommitAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	state := domain.NewState()
	campaign, err := domain.NewCampaign("c-1", "北区钻探", "北区", "CGCS2000", "负责人", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	state.Campaigns[campaign.ID] = campaign
	if err := store.Commit(state); err != nil {
		t.Fatal(err)
	}
	if store.Sequence() != 1 {
		t.Fatalf("序号=%d", store.Sequence())
	}
	restored, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Sequence() != 1 || restored.State().Campaigns["c-1"].Name != "北区钻探" {
		t.Fatal("恢复内容不一致")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("账本权限=%o", info.Mode().Perm())
	}
}

func TestOpenRejectsChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	state := domain.NewState()
	campaign, _ := domain.NewCampaign("c-1", "北区钻探", "北区", "CGCS2000", "负责人", time.Now())
	state.Campaigns[campaign.ID] = campaign
	if err := store.Commit(state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["checksum"] = "000000"
	data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("损坏账本未被拒绝")
	}
}

func TestTransactionDoesNotCommitNoChange(t *testing.T) {
	repo, err := New(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Transact(func(state *domain.State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if repo.Sequence() != 0 {
		t.Fatalf("无变化事务推进了序号: %d", repo.Sequence())
	}
}
