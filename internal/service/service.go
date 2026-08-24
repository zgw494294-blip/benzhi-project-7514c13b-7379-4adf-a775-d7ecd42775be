package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"corelog/internal/domain"
	"corelog/internal/repository"
	"corelog/internal/selfcheck"
)

type Service struct {
	repo         *repository.Repository
	clock        func() time.Time
	idgen        func(string) string
	anomalyCache map[string]anomalySummaryCacheEntry
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo, clock: time.Now, idgen: randomID, anomalyCache: make(map[string]anomalySummaryCacheEntry)}
}

func NewWithDependencies(repo *repository.Repository, clock func() time.Time, idgen func(string) string) *Service {
	return &Service{repo: repo, clock: clock, idgen: idgen, anomalyCache: make(map[string]anomalySummaryCacheEntry)}
}

func randomID(prefix string) string {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(bytes)
}

func requestHash(value any) (string, error) {
	payload := struct {
		JSON string `json:"json"`
		Full string `json:"full"`
	}{Full: fmt.Sprintf("%#v", value)}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("计算请求摘要失败: %w", err)
	}
	payload.JSON = string(encoded)
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("计算完整请求摘要失败: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func beginIdempotent(state *domain.State, key, operation, hash string) (string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, domain.Required("Idempotency-Key")
	}
	if len(key) > 128 {
		return "", false, domain.Invalid("Idempotency-Key", "长度不能超过 128")
	}
	record, exists := state.Idempotency[key]
	if !exists {
		return "", false, nil
	}
	if record.Operation != operation || record.RequestHash != hash {
		return "", false, domain.Conflict("幂等键已用于其他请求")
	}
	return record.ResourceID, true, nil
}

func saveIdempotent(state *domain.State, key, operation, hash, resourceID string, now time.Time) {
	state.Idempotency[key] = domain.IdempotencyRecord{Operation: operation, RequestHash: hash, ResourceID: resourceID, RecordedAt: now.UTC()}
}

func (s *Service) Snapshot() domain.State { return s.repo.Snapshot() }

func (s *Service) Selfcheck() selfcheck.Report { return selfcheck.Run(s.repo.Snapshot(), s.clock()) }

type CreateCampaignCommand struct {
	Name                string `json:"name"`
	Site                string `json:"site"`
	CoordinateReference string `json:"coordinateReference"`
	Coordinator         string `json:"coordinator"`
}

func (s *Service) CreateCampaign(command CreateCampaignCommand, key string) (domain.DrillingCampaign, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.DrillingCampaign{}, err
	}
	var result domain.DrillingCampaign
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "create_campaign", hash); err != nil {
			return err
		} else if found {
			campaign, ok := state.Campaigns[id]
			if !ok {
				return domain.Conflict("幂等记录指向不存在的任务")
			}
			result = campaign
			return nil
		}
		now := s.clock()
		campaign, err := domain.NewCampaign(s.idgen("campaign"), command.Name, command.Site, command.CoordinateReference, command.Coordinator, now)
		if err != nil {
			return err
		}
		state.Campaigns[campaign.ID] = campaign
		saveIdempotent(state, key, "create_campaign", hash, campaign.ID, now)
		result = campaign
		return nil
	})
	return result, err
}

func (s *Service) GetCampaign(id string) (domain.DrillingCampaign, error) {
	campaign, ok := s.repo.Snapshot().Campaigns[id]
	if !ok {
		return domain.DrillingCampaign{}, domain.NotFound("钻探任务")
	}
	return campaign, nil
}

func (s *Service) ListCampaigns() []domain.DrillingCampaign {
	state := s.repo.Snapshot()
	items := make([]domain.DrillingCampaign, 0, len(state.Campaigns))
	for _, campaign := range state.Campaigns {
		items = append(items, campaign)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

type AddIntervalCommand struct {
	CampaignID      string  `json:"-"`
	BoreholeCode    string  `json:"boreholeCode"`
	DepthStart      float64 `json:"depthStart"`
	DepthEnd        float64 `json:"depthEnd"`
	Lithology       string  `json:"lithology"`
	RecoveryRate    float64 `json:"recoveryRate"`
	Condition       string  `json:"condition"`
	ExpectedVersion int64   `json:"expectedVersion"`
}

func (s *Service) AddInterval(command AddIntervalCommand, key string) (domain.CoreInterval, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.CoreInterval{}, err
	}
	var result domain.CoreInterval
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "add_interval", hash); err != nil {
			return err
		} else if found {
			interval, ok := state.Intervals[id]
			if !ok {
				return domain.Conflict("幂等记录指向不存在的孔段")
			}
			result = interval
			return nil
		}
		campaign, ok := state.Campaigns[command.CampaignID]
		if !ok {
			return domain.NotFound("钻探任务")
		}
		if err := campaign.EnsureActive(); err != nil {
			return err
		}
		if err := campaign.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		now := s.clock()
		interval, err := domain.NewInterval(s.idgen("interval"), command.CampaignID, command.BoreholeCode, command.DepthStart, command.DepthEnd, command.Lithology, command.RecoveryRate, command.Condition, now)
		if err != nil {
			return err
		}
		existing := make([]domain.CoreInterval, 0)
		for _, item := range state.Intervals {
			existing = append(existing, item)
		}
		if err := domain.ValidateIntervalPlacement(interval, existing); err != nil {
			return err
		}
		campaign.Touch(now)
		state.Campaigns[campaign.ID] = campaign
		state.Intervals[interval.ID] = interval
		saveIdempotent(state, key, "add_interval", hash, interval.ID, now)
		result = interval
		return nil
	})
	return result, err
}

func (s *Service) ListIntervals(campaignID string) ([]domain.CoreInterval, error) {
	state := s.repo.Snapshot()
	if _, ok := state.Campaigns[campaignID]; !ok {
		return nil, domain.NotFound("钻探任务")
	}
	items := make([]domain.CoreInterval, 0)
	for _, interval := range state.Intervals {
		if interval.CampaignID == campaignID {
			items = append(items, interval)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].BoreholeCode == items[j].BoreholeCode {
			return items[i].DepthStart < items[j].DepthStart
		}
		return items[i].BoreholeCode < items[j].BoreholeCode
	})
	return items, nil
}

func (s *Service) GetInterval(id string) (domain.CoreInterval, error) {
	interval, ok := s.repo.Snapshot().Intervals[id]
	if !ok {
		return domain.CoreInterval{}, domain.NotFound("孔段")
	}
	return interval, nil
}

type AddAnomalyCommand struct {
	IntervalID      string `json:"-"`
	Kind            string `json:"kind"`
	Description     string `json:"description"`
	Evidence        string `json:"evidence"`
	ReportedBy      string `json:"reportedBy"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func (s *Service) AddAnomaly(command AddAnomalyCommand, key string) (domain.Anomaly, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.Anomaly{}, err
	}
	var result domain.Anomaly
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "add_anomaly", hash); err != nil {
			return err
		} else if found {
			for _, interval := range state.Intervals {
				for _, anomaly := range interval.Anomalies {
					if anomaly.ID == id {
						result = anomaly
						return nil
					}
				}
			}
			return domain.Conflict("幂等记录指向不存在的异常")
		}
		interval, ok := state.Intervals[command.IntervalID]
		if !ok {
			return domain.NotFound("孔段")
		}
		if err := interval.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		campaign := state.Campaigns[interval.CampaignID]
		if err := campaign.EnsureActive(); err != nil {
			return err
		}
		now := s.clock()
		anomaly, err := interval.AddAnomaly(s.idgen("anomaly"), command.Kind, command.Description, command.Evidence, command.ReportedBy, now)
		if err != nil {
			return err
		}
		campaign.Touch(now)
		state.Intervals[interval.ID] = interval
		state.Campaigns[campaign.ID] = campaign
		saveIdempotent(state, key, "add_anomaly", hash, anomaly.ID, now)
		result = anomaly
		return nil
	})
	return result, err
}

type ResolveAnomalyCommand struct {
	IntervalID      string `json:"-"`
	AnomalyID       string `json:"-"`
	Resolution      string `json:"resolution"`
	ResolvedBy      string `json:"resolvedBy"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func (s *Service) ResolveAnomaly(command ResolveAnomalyCommand, key string) (domain.Anomaly, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.Anomaly{}, err
	}
	var result domain.Anomaly
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "resolve_anomaly", hash); err != nil {
			return err
		} else if found {
			interval := state.Intervals[command.IntervalID]
			for _, anomaly := range interval.Anomalies {
				if anomaly.ID == id {
					result = anomaly
					return nil
				}
			}
			return domain.Conflict("幂等记录指向不存在的异常")
		}
		interval, ok := state.Intervals[command.IntervalID]
		if !ok {
			return domain.NotFound("孔段")
		}
		if err := interval.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		now := s.clock()
		anomaly, err := interval.ResolveAnomaly(command.AnomalyID, command.Resolution, command.ResolvedBy, now)
		if err != nil {
			return err
		}
		campaign := state.Campaigns[interval.CampaignID]
		campaign.Touch(now)
		state.Intervals[interval.ID] = interval
		state.Campaigns[campaign.ID] = campaign
		saveIdempotent(state, key, "resolve_anomaly", hash, anomaly.ID, now)
		result = anomaly
		return nil
	})
	return result, err
}
