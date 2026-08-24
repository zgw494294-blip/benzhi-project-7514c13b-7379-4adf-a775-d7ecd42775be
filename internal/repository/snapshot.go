package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"corelog/internal/domain"
)

type snapshotEnvelope struct {
	SchemaVersion int          `json:"schemaVersion"`
	Sequence      int64        `json:"sequence"`
	Checksum      string       `json:"checksum"`
	SavedAt       time.Time    `json:"savedAt"`
	State         domain.State `json:"state"`
}

type Store struct {
	path     string
	state    domain.State
	sequence int64
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("账本路径不能为空")
	}
	store := &Store{path: path, state: domain.NewState()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取账本失败: %w", err)
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("解析账本失败: %w", err)
	}
	if envelope.SchemaVersion != domain.SchemaVersion {
		return nil, fmt.Errorf("账本 schemaVersion=%d 不受支持", envelope.SchemaVersion)
	}
	if envelope.Sequence < 1 {
		return nil, errors.New("账本序号必须为正数")
	}
	if envelope.State.Campaigns == nil {
		envelope.State.Campaigns = make(map[string]domain.DrillingCampaign)
	}
	if envelope.State.Intervals == nil {
		envelope.State.Intervals = make(map[string]domain.CoreInterval)
	}
	if envelope.State.Sampling == nil {
		envelope.State.Sampling = make(map[string]domain.SamplingRequest)
	}
	if envelope.State.TestResults == nil {
		envelope.State.TestResults = make(map[string]domain.TestResult)
	}
	if envelope.State.Certificates == nil {
		envelope.State.Certificates = make(map[string]domain.CustodyCertificate)
	}
	if envelope.State.Idempotency == nil {
		envelope.State.Idempotency = make(map[string]domain.IdempotencyRecord)
	}
	if err := validateState(envelope.State); err != nil {
		return nil, err
	}
	checksum, err := stateChecksum(envelope.State)
	if err != nil {
		return nil, err
	}
	if checksum != envelope.Checksum {
		return nil, errors.New("账本校验和不匹配")
	}
	store.state = envelope.State
	store.sequence = envelope.Sequence
	return store, nil
}

func validateState(state domain.State) error {
	if state.Campaigns == nil || state.Intervals == nil || state.Sampling == nil || state.TestResults == nil || state.Certificates == nil || state.Idempotency == nil {
		return errors.New("账本缺少必要索引")
	}
	for id, campaign := range state.Campaigns {
		if id == "" || campaign.ID != id || campaign.Version < 1 {
			return errors.New("账本中的任务索引无效")
		}
	}
	for id, interval := range state.Intervals {
		if id == "" || interval.ID != id || interval.Version < 1 {
			return errors.New("账本中的孔段索引无效")
		}
		if _, ok := state.Campaigns[interval.CampaignID]; !ok {
			return errors.New("账本中的孔段引用不存在任务")
		}
		for _, anomaly := range interval.Anomalies {
			if err := anomaly.ValidateEvidence(); err != nil {
				return err
			}
		}
	}
	for id, request := range state.Sampling {
		if id == "" || request.ID != id || request.Version < 1 {
			return errors.New("账本中的取样申请索引无效")
		}
		if _, ok := state.Campaigns[request.CampaignID]; !ok {
			return errors.New("账本中的取样申请引用不存在任务")
		}
		for _, intervalID := range request.IntervalIDs {
			interval, ok := state.Intervals[intervalID]
			if !ok || interval.CampaignID != request.CampaignID {
				return errors.New("账本中的取样申请引用无效孔段")
			}
		}
	}
	for id, result := range state.TestResults {
		if id == "" || result.ID != id || result.Version < 1 {
			return errors.New("账本中的检测结果索引无效")
		}
		if _, ok := state.Sampling[result.SamplingRequestID]; !ok {
			return errors.New("账本中的检测结果引用不存在申请")
		}
	}
	for id, certificate := range state.Certificates {
		if id == "" || certificate.ID != id || certificate.SchemaVersion != domain.SchemaVersion {
			return errors.New("账本中的凭据索引无效")
		}
		if _, ok := state.Sampling[certificate.SamplingRequestID]; !ok {
			return errors.New("账本中的凭据引用不存在申请")
		}
		valid, err := domain.VerifyCertificate(certificate)
		if err != nil || !valid {
			return errors.New("账本中的凭据校验失败")
		}
	}
	return nil
}

func stateChecksum(state domain.State) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("序列化账本失败: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Sequence() int64 { return s.sequence }

func (s *Store) State() domain.State { return s.state.Clone() }

func (s *Store) Commit(next domain.State) error {
	if err := validateState(next); err != nil {
		return err
	}
	checksum, err := stateChecksum(next)
	if err != nil {
		return err
	}
	nextSequence := s.sequence + 1
	if nextSequence < 1 {
		nextSequence = 1
	}
	envelope := snapshotEnvelope{SchemaVersion: domain.SchemaVersion, Sequence: nextSequence, Checksum: checksum, SavedAt: time.Now().UTC(), State: next}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("编码账本失败: %w", err)
	}
	if err := atomicWrite(s.path, data); err != nil {
		return err
	}
	s.state = next.Clone()
	s.sequence = nextSequence
	return nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建账本目录失败: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".corelog-snapshot-*")
	if err != nil {
		return fmt.Errorf("创建临时账本失败: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("设置账本权限失败: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("写入临时账本失败: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("同步临时账本失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭临时账本失败: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("替换账本失败: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("打开账本目录失败: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("同步账本目录失败: %w", err)
	}
	return nil
}
