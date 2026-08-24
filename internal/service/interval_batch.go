package service

import (
	"strconv"
	"strings"

	"corelog/internal/domain"
)

type BatchIntervalItem struct {
	ID           string  `json:"id,omitempty"`
	IntervalID   string  `json:"intervalID,omitempty"`
	CampaignID   string  `json:"campaignID,omitempty"`
	BoreholeCode string  `json:"boreholeCode"`
	DepthStart   float64 `json:"depthStart"`
	DepthEnd     float64 `json:"depthEnd"`
	Lithology    string  `json:"lithology"`
	RecoveryRate float64 `json:"recoveryRate"`
	Condition    string  `json:"condition"`
}

type BatchIntervalsCommand struct {
	CampaignID      string              `json:"-"`
	Intervals       []BatchIntervalItem `json:"intervals"`
	ExpectedVersion int64               `json:"expectedVersion"`
}

type BatchIntervalsResult struct {
	Intervals []domain.CoreInterval     `json:"intervals"`
	Progress  []domain.BoreholeProgress `json:"progress"`
}

func (s *Service) AddIntervalsBatch(command BatchIntervalsCommand, key string) (BatchIntervalsResult, error) {
	hash, err := requestHash(command)
	if err != nil {
		return BatchIntervalsResult{}, err
	}
	var result BatchIntervalsResult
	err = s.repo.Transact(func(state *domain.State) error {
		if resource, found, err := beginIdempotent(state, key, "add_intervals_batch", hash); err != nil {
			return err
		} else if found {
			ids := strings.Split(resource, ",")
			for _, id := range ids {
				interval, ok := state.Intervals[id]
				if !ok {
					return domain.Conflict("幂等记录指向不存在的孔段")
				}
				result.Intervals = append(result.Intervals, interval)
			}
			result.Progress = progressForCampaign(*state, command.CampaignID)
			return nil
		}
		if len(command.Intervals) == 0 {
			return domain.Invalid("intervals", "批量孔段不能为空")
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
		candidates := make([]domain.CoreInterval, 0, len(command.Intervals))
		for idx, item := range command.Intervals {
			if strings.TrimSpace(item.CampaignID) != "" && strings.TrimSpace(item.CampaignID) != command.CampaignID {
				return domain.Invalid("intervals["+strconv.Itoa(idx)+"].campaignID", "孔段引用了其他钻探任务")
			}
			intervalID := strings.TrimSpace(item.ID)
			if intervalID == "" {
				intervalID = strings.TrimSpace(item.IntervalID)
			}
			if intervalID == "" {
				intervalID = s.idgen("interval")
			}
			if _, exists := state.Intervals[intervalID]; exists {
				return domain.Conflict("孔段标识已经存在")
			}
			for _, candidate := range candidates {
				if candidate.ID == intervalID {
					return domain.Invalid("intervals["+strconv.Itoa(idx)+"].id", "不能包含重复孔段标识")
				}
			}
			interval, err := domain.NewInterval(intervalID, command.CampaignID, item.BoreholeCode, item.DepthStart, item.DepthEnd, item.Lithology, item.RecoveryRate, item.Condition, now)
			if err != nil {
				if e, ok := err.(*domain.Error); ok {
					return domain.NewError(e.Code, e.Message, "intervals["+strconv.Itoa(idx)+"]."+e.Field)
				}
				return err
			}
			candidates = append(candidates, interval)
		}
		existing := make([]domain.CoreInterval, 0, len(state.Intervals))
		for _, interval := range state.Intervals {
			existing = append(existing, interval)
		}
		if err := domain.ValidateIntervalBatch(candidates, existing); err != nil {
			return err
		}
		campaign.Touch(now)
		state.Campaigns[campaign.ID] = campaign
		ids := make([]string, 0, len(candidates))
		for _, interval := range candidates {
			state.Intervals[interval.ID] = interval
			ids = append(ids, interval.ID)
			result.Intervals = append(result.Intervals, interval)
		}
		saveIdempotent(state, key, "add_intervals_batch", hash, strings.Join(ids, ","), now)
		result.Progress = progressForCampaign(*state, command.CampaignID)
		return nil
	})
	return result, err
}

func progressForCampaign(state domain.State, campaignID string) []domain.BoreholeProgress {
	intervals := make([]domain.CoreInterval, 0)
	for _, interval := range state.Intervals {
		if interval.CampaignID == campaignID {
			intervals = append(intervals, interval)
		}
	}
	return domain.SummarizeIntervalProgress(intervals)
}

func (s *Service) GetIntervalProgress(campaignID string) ([]domain.BoreholeProgress, error) {
	state := s.repo.Snapshot()
	if _, ok := state.Campaigns[campaignID]; !ok {
		return nil, domain.NotFound("钻探任务")
	}
	return progressForCampaign(state, campaignID), nil
}
