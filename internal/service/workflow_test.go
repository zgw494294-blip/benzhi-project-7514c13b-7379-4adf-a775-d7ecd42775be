package service

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"corelog/internal/domain"
	"corelog/internal/repository"
)

type fixture struct {
	service *Service
	repo    *repository.Repository
	path    string
	next    int
	now     time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.json")
	repo, err := repository.New(path)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{repo: repo, path: path, now: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)}
	f.service = NewWithDependencies(repo, func() time.Time { f.now = f.now.Add(time.Second); return f.now }, func(prefix string) string { f.next++; return fmt.Sprintf("%s-%d", prefix, f.next) })
	return f
}

func TestCompleteWorkflowWithReturnAndResubmit(t *testing.T) {
	f := newFixture(t)
	campaign, err := f.service.CreateCampaign(CreateCampaignCommand{Name: "北区深部钻探", Site: "北区 3 号平台", CoordinateReference: "CGCS2000", Coordinator: "王工"}, "key-campaign")
	if err != nil {
		t.Fatal(err)
	}
	interval, err := f.service.AddInterval(AddIntervalCommand{CampaignID: campaign.ID, BoreholeCode: "ZK01", DepthStart: 0, DepthEnd: 12.5, Lithology: "花岗闪长岩", RecoveryRate: 96.4, Condition: "局部裂隙", ExpectedVersion: campaign.Version}, "key-interval")
	if err != nil {
		t.Fatal(err)
	}
	anomaly, err := f.service.AddAnomaly(AddAnomalyCommand{IntervalID: interval.ID, Kind: "破碎带", Description: "10.2m 至 10.6m 破碎", Evidence: "现场照片 IMG-203", ReportedBy: "张编录", ExpectedVersion: interval.Version}, "key-anomaly")
	if err != nil {
		t.Fatal(err)
	}
	interval, _ = f.service.GetInterval(interval.ID)
	campaign, _ = f.service.GetCampaign(campaign.ID)
	request, err := f.service.CreateSamplingRequest(CreateSamplingCommand{CampaignID: campaign.ID, IntervalIDs: []string{interval.ID}, IntervalVersions: map[string]int64{interval.ID: interval.Version}, Purpose: "主量元素检测", RequestedBy: "项目负责人", ExpectedVersion: campaign.Version}, "key-request")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.service.ReviewSamplingRequest(ReviewSamplingCommand{RequestID: request.ID, Decision: "approve", Reviewer: "复核员", ReviewNote: "尝试批准", ExpectedVersion: request.Version}, "key-review-rejected")
	if err == nil {
		t.Fatal("未处置异常的申请被批准")
	}
	request, err = f.service.ReviewSamplingRequest(ReviewSamplingCommand{RequestID: request.ID, Decision: "return", Reviewer: "复核员", ReviewNote: "请先处置破碎带异常", ExpectedVersion: request.Version}, "key-return")
	if err != nil {
		t.Fatal(err)
	}
	interval, _ = f.service.GetInterval(interval.ID)
	if interval.FrozenBy != "" {
		t.Fatal("退回申请后孔段未解冻")
	}
	_, err = f.service.ResolveAnomaly(ResolveAnomalyCommand{IntervalID: interval.ID, AnomalyID: anomaly.ID, Resolution: "补充取芯照片并界定破碎范围", ResolvedBy: "王工", ExpectedVersion: interval.Version}, "key-resolve")
	if err != nil {
		t.Fatal(err)
	}
	interval, _ = f.service.GetInterval(interval.ID)
	request, err = f.service.ResubmitSamplingRequest(ResubmitSamplingCommand{RequestID: request.ID, IntervalVersions: map[string]int64{interval.ID: interval.Version}, Purpose: "主量元素检测，已补充异常处置", RequestedBy: "项目负责人", ExpectedVersion: request.Version}, "key-resubmit")
	if err != nil {
		t.Fatal(err)
	}
	request, err = f.service.ReviewSamplingRequest(ReviewSamplingCommand{RequestID: request.ID, Decision: "approve", Reviewer: "质量复核员", ReviewNote: "异常处置及取样范围完整", ExpectedVersion: request.Version}, "key-approve")
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.service.RecordTestResult(RecordTestResultCommand{RequestID: request.ID, SampleCode: "ZK01-S001", Method: "XRF", Measurements: map[string]float64{"SiO2": 67.31, "Al2O3": 15.4}, Unit: "%", PerformedBy: "实验员", ExpectedVersion: request.Version}, "key-result")
	if err != nil {
		t.Fatal(err)
	}
	result, err = f.service.ReviewTestResult(ReviewTestResultCommand{ResultID: result.ID, Decision: "approve", Reviewer: "质量复核员", ReviewNote: "质控样偏差符合要求", ExpectedVersion: result.Version}, "key-result-review")
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := f.service.IssueCertificate(IssueCertificateCommand{RequestID: request.ID, IssuedBy: "实验室接样员", ExpectedVersion: request.Version}, "key-certificate")
	if err != nil {
		t.Fatal(err)
	}
	verification, err := f.service.VerifyCertificate(certificate.ID)
	if err != nil || !verification.Valid {
		t.Fatalf("凭据验证失败: %+v %v", verification, err)
	}
	if report := f.service.Selfcheck(); !report.Passed {
		t.Fatalf("最终自检失败: %+v", report.Failures)
	}
	restoredRepo, err := repository.New(f.path)
	if err != nil {
		t.Fatal(err)
	}
	restored := New(restoredRepo)
	storedCertificate, err := restored.GetCertificate(certificate.ID)
	if err != nil || storedCertificate.PayloadHash != certificate.PayloadHash {
		t.Fatal("重启后凭据不一致")
	}
	storedCampaign, _ := restored.GetCampaign(campaign.ID)
	if storedCampaign.Status != domain.CampaignCustodyIssued {
		t.Fatal("任务未进入完成交接状态")
	}
}

func TestIdempotencyAndOptimisticVersion(t *testing.T) {
	f := newFixture(t)
	command := CreateCampaignCommand{Name: "测试任务", Site: "平台", CoordinateReference: "CGCS2000", Coordinator: "负责人"}
	first, err := f.service.CreateCampaign(command, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	sequence := f.repo.Sequence()
	second, err := f.service.CreateCampaign(command, "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("幂等请求创建了不同资源")
	}
	if f.repo.Sequence() != sequence {
		t.Fatal("幂等重放仍写入账本")
	}
	changed := command
	changed.Site = "其他平台"
	if _, err := f.service.CreateCampaign(changed, "same-key"); err == nil {
		t.Fatal("幂等键复用未冲突")
	}
	_, err = f.service.AddInterval(AddIntervalCommand{CampaignID: first.ID, BoreholeCode: "ZK01", DepthStart: 0, DepthEnd: 10, Lithology: "花岗岩", RecoveryRate: 90, Condition: "完整", ExpectedVersion: 99}, "bad-version")
	if err == nil {
		t.Fatal("错误 expectedVersion 未被拒绝")
	}
}

func TestRequestHashIncludesPathResource(t *testing.T) {
	first := AddIntervalCommand{CampaignID: "campaign-1", BoreholeCode: "ZK01", DepthStart: 0, DepthEnd: 10, Lithology: "花岗岩", RecoveryRate: 90, Condition: "完整", ExpectedVersion: 1}
	second := first
	second.CampaignID = "campaign-2"
	firstHash, err := requestHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := requestHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == secondHash {
		t.Fatal("不同路径资源产生了相同请求摘要")
	}
}
