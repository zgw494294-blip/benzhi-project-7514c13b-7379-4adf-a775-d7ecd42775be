package service

import (
	"sort"
	"strconv"
	"strings"

	"corelog/internal/domain"
	"corelog/internal/selfcheck"
)

type CreateSamplingCommand struct {
	CampaignID       string           `json:"-"`
	IntervalIDs      []string         `json:"intervalIDs"`
	IntervalVersions map[string]int64 `json:"intervalVersions"`
	Purpose          string           `json:"purpose"`
	RequestedBy      string           `json:"requestedBy"`
	ExpectedVersion  int64            `json:"expectedVersion"`
}

func (s *Service) CreateSamplingRequest(command CreateSamplingCommand, key string) (domain.SamplingRequest, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.SamplingRequest{}, err
	}
	var result domain.SamplingRequest
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "create_sampling", hash); err != nil {
			return err
		} else if found {
			request, ok := state.Sampling[id]
			if !ok {
				return domain.Conflict("幂等记录指向不存在的取样申请")
			}
			result = request
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
		request, err := domain.NewSamplingRequest(s.idgen("sampling"), command.CampaignID, command.IntervalIDs, command.IntervalVersions, command.Purpose, command.RequestedBy, now)
		if err != nil {
			return err
		}
		for _, intervalID := range request.IntervalIDs {
			interval, ok := state.Intervals[intervalID]
			if !ok || interval.CampaignID != command.CampaignID {
				return domain.Invalid("intervalIDs", "包含不属于当前任务的孔段")
			}
			if interval.Version != request.IntervalVersions[intervalID] {
				return domain.Conflict("孔段版本已变化")
			}
			if err := interval.Freeze(request.ID); err != nil {
				return err
			}
			state.Intervals[interval.ID] = interval
		}
		campaign.Touch(now)
		state.Campaigns[campaign.ID] = campaign
		state.Sampling[request.ID] = request
		saveIdempotent(state, key, "create_sampling", hash, request.ID, now)
		result = request
		return nil
	})
	return result, err
}

func (s *Service) GetSamplingRequest(id string) (domain.SamplingRequest, error) {
	request, ok := s.repo.Snapshot().Sampling[id]
	if !ok {
		return domain.SamplingRequest{}, domain.NotFound("取样申请")
	}
	return request, nil
}

func (s *Service) ListSamplingRequests(campaignID string) ([]domain.SamplingRequest, error) {
	state := s.repo.Snapshot()
	if _, ok := state.Campaigns[campaignID]; !ok {
		return nil, domain.NotFound("钻探任务")
	}
	items := make([]domain.SamplingRequest, 0)
	for _, request := range state.Sampling {
		if request.CampaignID == campaignID {
			items = append(items, request)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

type ReviewSamplingCommand struct {
	RequestID       string `json:"-"`
	Decision        string `json:"decision"`
	Reviewer        string `json:"reviewer"`
	ReviewNote      string `json:"reviewNote"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func (s *Service) ReviewSamplingRequest(command ReviewSamplingCommand, key string) (domain.SamplingRequest, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.SamplingRequest{}, err
	}
	if command.Decision != "approve" && command.Decision != "return" {
		return domain.SamplingRequest{}, domain.Invalid("decision", "必须为 approve 或 return")
	}
	var result domain.SamplingRequest
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "review_sampling", hash); err != nil {
			return err
		} else if found {
			result = state.Sampling[id]
			return nil
		}
		request, ok := state.Sampling[command.RequestID]
		if !ok {
			return domain.NotFound("取样申请")
		}
		if err := request.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		if command.Decision == "approve" {
			for _, intervalID := range request.IntervalIDs {
				interval := state.Intervals[intervalID]
				if interval.FrozenBy != request.ID || interval.FrozenVersion != request.IntervalVersions[intervalID] {
					return domain.Conflict("冻结孔段版本与申请不一致")
				}
				if interval.HasOpenAnomalies() {
					return domain.StateConflict("存在未处置异常，不能批准取样")
				}
			}
		}
		now := s.clock()
		if err := request.Review(command.Decision == "approve", command.Reviewer, command.ReviewNote, now); err != nil {
			return err
		}
		if command.Decision == "return" {
			for _, intervalID := range request.IntervalIDs {
				interval := state.Intervals[intervalID]
				interval.Unfreeze(request.ID)
				state.Intervals[intervalID] = interval
			}
		}
		state.Sampling[request.ID] = request
		saveIdempotent(state, key, "review_sampling", hash, request.ID, now)
		result = request
		return nil
	})
	return result, err
}

type ResubmitSamplingCommand struct {
	RequestID        string           `json:"-"`
	IntervalVersions map[string]int64 `json:"intervalVersions"`
	Purpose          string           `json:"purpose"`
	RequestedBy      string           `json:"requestedBy"`
	ExpectedVersion  int64            `json:"expectedVersion"`
}

func (s *Service) ResubmitSamplingRequest(command ResubmitSamplingCommand, key string) (domain.SamplingRequest, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.SamplingRequest{}, err
	}
	var result domain.SamplingRequest
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "resubmit_sampling", hash); err != nil {
			return err
		} else if found {
			result = state.Sampling[id]
			return nil
		}
		request, ok := state.Sampling[command.RequestID]
		if !ok {
			return domain.NotFound("取样申请")
		}
		if err := request.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		for _, intervalID := range request.IntervalIDs {
			interval, ok := state.Intervals[intervalID]
			if !ok || interval.CampaignID != request.CampaignID {
				return domain.Invalid("intervalVersions", "孔段引用已经失效")
			}
			if interval.Version != command.IntervalVersions[intervalID] {
				return domain.Conflict("补正孔段版本已变化")
			}
			if interval.HasOpenAnomalies() {
				return domain.StateConflict("补正重提前必须处置所有异常")
			}
			if err := interval.Freeze(request.ID); err != nil {
				return err
			}
			state.Intervals[intervalID] = interval
		}
		now := s.clock()
		if err := request.Resubmit(command.IntervalVersions, command.Purpose, command.RequestedBy, now); err != nil {
			return err
		}
		state.Sampling[request.ID] = request
		saveIdempotent(state, key, "resubmit_sampling", hash, request.ID, now)
		result = request
		return nil
	})
	return result, err
}

type RecordTestResultCommand struct {
	RequestID       string             `json:"-"`
	SampleCode      string             `json:"sampleCode"`
	Method          string             `json:"method"`
	Measurements    map[string]float64 `json:"measurements"`
	Unit            string             `json:"unit"`
	PerformedBy     string             `json:"performedBy"`
	ExpectedVersion int64              `json:"expectedVersion"`
}

func (s *Service) RecordTestResult(command RecordTestResultCommand, key string) (domain.TestResult, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.TestResult{}, err
	}
	var result domain.TestResult
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "record_test_result", hash); err != nil {
			return err
		} else if found {
			result = state.TestResults[id]
			return nil
		}
		request, ok := state.Sampling[command.RequestID]
		if !ok {
			return domain.NotFound("取样申请")
		}
		if err := request.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		if request.Status != domain.SamplingApproved {
			return domain.StateConflict("只有已批准申请可以登记检测结果")
		}
		for _, existing := range state.TestResults {
			if existing.SamplingRequestID == request.ID && existing.SampleCode == strings.TrimSpace(command.SampleCode) {
				return domain.Conflict("样品编号已经登记检测结果")
			}
		}
		now := s.clock()
		testResult, err := domain.NewTestResult(s.idgen("result"), request.ID, command.SampleCode, command.Method, command.Measurements, command.Unit, command.PerformedBy, now)
		if err != nil {
			return err
		}
		state.TestResults[testResult.ID] = testResult
		saveIdempotent(state, key, "record_test_result", hash, testResult.ID, now)
		result = testResult
		return nil
	})
	if err == nil {
		s.refreshHandoffReadiness(command.RequestID)
	}
	return result, err
}

func (s *Service) GetTestResult(id string) (domain.TestResult, error) {
	result, ok := s.repo.Snapshot().TestResults[id]
	if !ok {
		return domain.TestResult{}, domain.NotFound("检测结果")
	}
	return result, nil
}

func (s *Service) ListTestResults(requestID string) ([]domain.TestResult, error) {
	state := s.repo.Snapshot()
	if _, ok := state.Sampling[requestID]; !ok {
		return nil, domain.NotFound("取样申请")
	}
	items := make([]domain.TestResult, 0)
	for _, result := range state.TestResults {
		if result.SamplingRequestID == requestID {
			items = append(items, result)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RecordedAt.Before(items[j].RecordedAt) })
	return items, nil
}

type ReviewTestResultCommand struct {
	ResultID        string `json:"-"`
	Decision        string `json:"decision"`
	Reviewer        string `json:"reviewer"`
	ReviewNote      string `json:"reviewNote"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

type BatchReviewItem struct {
	ResultID        string `json:"resultID"`
	Decision        string `json:"decision"`
	Reviewer        string `json:"reviewer"`
	ReviewNote      string `json:"reviewNote"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

type BatchReviewResult struct {
	SamplingRequestID string                  `json:"samplingRequestID"`
	Results           []domain.TestResult     `json:"results"`
	Readiness         domain.HandoffReadiness `json:"readiness"`
}

func (s *Service) ReviewTestResultsBatch(requestID string, items []BatchReviewItem, key string) (BatchReviewResult, error) {
	command := struct {
		RequestID string            `json:"requestID"`
		Items     []BatchReviewItem `json:"items"`
	}{requestID, items}
	hash, err := requestHash(command)
	if err != nil {
		return BatchReviewResult{}, err
	}
	var result BatchReviewResult
	err = s.repo.Transact(func(state *domain.State) error {
		if resource, found, err := beginIdempotent(state, key, "review_test_results_batch", hash); err != nil {
			return err
		} else if found {
			if _, ok := state.Sampling[resource]; !ok {
				return domain.Conflict("幂等记录指向不存在的取样申请")
			}
			result = batchReviewSnapshot(*state, resource)
			return nil
		}
		request, ok := state.Sampling[requestID]
		if !ok {
			return domain.NotFound("取样申请")
		}
		if request.Status != domain.SamplingApproved {
			return domain.StateConflict("只有已批准申请可以复核检测结果")
		}
		if len(items) == 0 {
			return domain.Invalid("items", "批量复核不能为空")
		}
		seen := make(map[string]bool, len(items))
		for idx, item := range items {
			if strings.TrimSpace(item.ResultID) == "" {
				return domain.Invalid("items", "检测结果编号不能为空")
			}
			if seen[item.ResultID] {
				return domain.Invalid("items", "不能重复复核同一检测结果")
			}
			seen[item.ResultID] = true
			if item.Decision != "approve" && item.Decision != "return" {
				return domain.Invalid("items["+strconv.Itoa(idx)+"].decision", "必须为 approve 或 return")
			}
			res, ok := state.TestResults[item.ResultID]
			if !ok {
				return domain.Conflict("批量中包含不存在的检测结果")
			}
			if res.SamplingRequestID != request.ID {
				return domain.Conflict("批量检测结果不属于当前取样申请")
			}
			if err := res.CheckVersion(item.ExpectedVersion); err != nil {
				return err
			}
			if res.ReviewStatus != domain.ReviewPending && res.ReviewStatus != domain.ReviewReturned {
				return domain.StateConflict("检测结果当前状态不可复核")
			}
			if strings.TrimSpace(item.Reviewer) == "" {
				return domain.Required("items[" + strconv.Itoa(idx) + "].reviewer")
			}
			if strings.TrimSpace(item.ReviewNote) == "" {
				return domain.Required("items[" + strconv.Itoa(idx) + "].reviewNote")
			}
		}
		now := s.clock()
		for _, item := range items {
			res := state.TestResults[item.ResultID]
			if err := res.Review(item.Decision == "approve", item.Reviewer, item.ReviewNote, now); err != nil {
				return err
			}
			state.TestResults[res.ID] = res
		}
		saveIdempotent(state, key, "review_test_results_batch", hash, request.ID, now)
		result = batchReviewSnapshot(*state, request.ID)
		return nil
	})
	if err == nil {
		s.refreshHandoffReadiness(requestID)
	}
	return result, err
}

func batchReviewSnapshot(state domain.State, requestID string) BatchReviewResult {
	results := make([]domain.TestResult, 0)
	for _, item := range state.TestResults {
		if item.SamplingRequestID == requestID {
			results = append(results, item)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].RecordedAt.Before(results[j].RecordedAt) })
	return BatchReviewResult{SamplingRequestID: requestID, Results: results, Readiness: domain.CalculateHandoffReadiness(requestID, results)}
}

func cloneHandoffReadiness(readiness domain.HandoffReadiness) domain.HandoffReadiness {
	readiness.BlockingResults = append([]string(nil), readiness.BlockingResults...)
	return readiness
}

func (s *Service) loadHandoffReadiness(requestID string) (domain.HandoffReadiness, bool) {
	s.readinessMu.RLock()
	defer s.readinessMu.RUnlock()
	readiness, ok := s.readinessCache[requestID]
	return cloneHandoffReadiness(readiness), ok
}

// invalidateHandoffReadiness drops the cached readiness for a request so that
// the next read recomputes it from the committed ledger state.
func (s *Service) invalidateHandoffReadiness(requestID string) {
	s.readinessMu.Lock()
	defer s.readinessMu.Unlock()
	delete(s.readinessCache, requestID)
}

// refreshHandoffReadiness recomputes and caches the readiness for a request
// from the latest committed ledger state. It is called after any successful
// operation that can change handoff readiness so that subsequent reads never
// observe a stale cached value. When the request no longer exists the cache
// entry is dropped.
func (s *Service) refreshHandoffReadiness(requestID string) {
	state := s.repo.Snapshot()
	if _, ok := state.Sampling[requestID]; !ok {
		s.invalidateHandoffReadiness(requestID)
		return
	}
	s.readinessMu.Lock()
	defer s.readinessMu.Unlock()
	s.readinessCache[requestID] = cloneHandoffReadiness(batchReviewSnapshot(state, requestID).Readiness)
}

func (s *Service) HandoffReadiness(requestID string) (domain.HandoffReadiness, error) {
	state := s.repo.Snapshot()
	if _, ok := state.Sampling[requestID]; !ok {
		return domain.HandoffReadiness{}, domain.NotFound("取样申请")
	}
	if readiness, ok := s.loadHandoffReadiness(requestID); ok {
		return readiness, nil
	}
	// Re-check under the write lock to avoid racing concurrent readers or a
	// refresh from recomputing and overwriting a just-populated entry.
	s.readinessMu.Lock()
	if readiness, ok := s.readinessCache[requestID]; ok {
		s.readinessMu.Unlock()
		return cloneHandoffReadiness(readiness), nil
	}
	readiness := cloneHandoffReadiness(batchReviewSnapshot(state, requestID).Readiness)
	s.readinessCache[requestID] = readiness
	s.readinessMu.Unlock()
	return cloneHandoffReadiness(readiness), nil
}

func (s *Service) ReviewTestResult(command ReviewTestResultCommand, key string) (domain.TestResult, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.TestResult{}, err
	}
	if command.Decision != "approve" && command.Decision != "return" {
		return domain.TestResult{}, domain.Invalid("decision", "必须为 approve 或 return")
	}
	var result domain.TestResult
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "review_test_result", hash); err != nil {
			return err
		} else if found {
			result = state.TestResults[id]
			return nil
		}
		testResult, ok := state.TestResults[command.ResultID]
		if !ok {
			return domain.NotFound("检测结果")
		}
		if err := testResult.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		if err := testResult.Review(command.Decision == "approve", command.Reviewer, command.ReviewNote, s.clock()); err != nil {
			return err
		}
		state.TestResults[testResult.ID] = testResult
		saveIdempotent(state, key, "review_test_result", hash, testResult.ID, s.clock())
		result = testResult
		return nil
	})
	if err == nil {
		s.refreshHandoffReadiness(result.SamplingRequestID)
	}
	return result, err
}

type IssueCertificateCommand struct {
	RequestID       string `json:"-"`
	IssuedBy        string `json:"issuedBy"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func (s *Service) IssueCertificate(command IssueCertificateCommand, key string) (domain.CustodyCertificate, error) {
	hash, err := requestHash(command)
	if err != nil {
		return domain.CustodyCertificate{}, err
	}
	var result domain.CustodyCertificate
	err = s.repo.Transact(func(state *domain.State) error {
		if id, found, err := beginIdempotent(state, key, "issue_certificate", hash); err != nil {
			return err
		} else if found {
			result = state.Certificates[id]
			return nil
		}
		request, ok := state.Sampling[command.RequestID]
		if !ok {
			return domain.NotFound("取样申请")
		}
		if err := request.CheckVersion(command.ExpectedVersion); err != nil {
			return err
		}
		if request.Status != domain.SamplingApproved {
			return domain.StateConflict("只有已批准申请可以签发凭据")
		}
		sampleCodes := make([]string, 0)
		resultsForRequest := make([]domain.TestResult, 0)
		for _, testResult := range state.TestResults {
			if testResult.SamplingRequestID != request.ID {
				continue
			}
			resultsForRequest = append(resultsForRequest, testResult)
			sampleCodes = append(sampleCodes, testResult.SampleCode)
		}
		readiness := domain.CalculateHandoffReadiness(request.ID, resultsForRequest)
		if !readiness.CanIssue {
			return domain.StateConflict("交接准备未完成，阻断检测结果: " + strings.Join(readiness.BlockingResults, ","))
		}
		preflight := selfcheck.Run(*state, s.clock())
		if !preflight.Passed {
			return domain.StateConflict("交接前自检未通过: " + strings.Join(preflight.Failures, ","))
		}
		now := s.clock()
		certificate, err := domain.NewCertificate(s.idgen("certificate"), request.CampaignID, request.ID, sampleCodes, command.IssuedBy, now, int64(len(state.Certificates)+1))
		if err != nil {
			return err
		}
		if err := request.Close(now); err != nil {
			return err
		}
		campaign := state.Campaigns[request.CampaignID]
		if err := campaign.MarkCustodyIssued(now); err != nil {
			return err
		}
		state.Sampling[request.ID] = request
		state.Campaigns[campaign.ID] = campaign
		state.Certificates[certificate.ID] = certificate
		saveIdempotent(state, key, "issue_certificate", hash, certificate.ID, now)
		result = certificate
		return nil
	})
	if err == nil {
		s.refreshHandoffReadiness(command.RequestID)
	}
	return result, err
}

func (s *Service) GetCertificate(id string) (domain.CustodyCertificate, error) {
	certificate, ok := s.repo.Snapshot().Certificates[id]
	if !ok {
		return domain.CustodyCertificate{}, domain.NotFound("交接凭据")
	}
	return certificate, nil
}

type CertificateVerification struct {
	CertificateID string `json:"certificateID"`
	Valid         bool   `json:"valid"`
	PayloadHash   string `json:"payloadHash"`
}

func (s *Service) VerifyCertificate(id string) (CertificateVerification, error) {
	certificate, err := s.GetCertificate(id)
	if err != nil {
		return CertificateVerification{}, err
	}
	valid, err := domain.VerifyCertificate(certificate)
	if err != nil {
		return CertificateVerification{}, err
	}
	return CertificateVerification{CertificateID: id, Valid: valid, PayloadHash: certificate.PayloadHash}, nil
}
