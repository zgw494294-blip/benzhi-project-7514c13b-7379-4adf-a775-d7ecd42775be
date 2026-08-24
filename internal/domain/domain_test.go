package domain

import (
	"testing"
	"time"
)

func TestValidateIntervalPlacement(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	first, err := NewInterval("i-1", "c-1", "ZK01", 0, 10, "花岗岩", 95, "完整", now)
	if err != nil {
		t.Fatal(err)
	}
	continuous, err := NewInterval("i-2", "c-1", "ZK01", 10, 20, "片麻岩", 91, "裂隙", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateIntervalPlacement(continuous, []CoreInterval{first}); err != nil {
		t.Fatalf("连续孔段被拒绝: %v", err)
	}
	overlap, _ := NewInterval("i-3", "c-1", "ZK01", 9, 12, "石英岩", 88, "完整", now)
	if err := ValidateIntervalPlacement(overlap, []CoreInterval{first}); err == nil {
		t.Fatal("预期重叠校验失败")
	}
	gap, _ := NewInterval("i-4", "c-1", "ZK01", 11, 12, "石英岩", 88, "完整", now)
	if err := ValidateIntervalPlacement(gap, []CoreInterval{first}); err == nil {
		t.Fatal("预期间断校验失败")
	}
}

func TestAnomalyLifecycleAndFreeze(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	interval, err := NewInterval("i-1", "c-1", "ZK01", 0, 10, "花岗岩", 95, "完整", now)
	if err != nil {
		t.Fatal(err)
	}
	anomaly, err := interval.AddAnomaly("a-1", "破碎", "局部破碎带", "照片 IMG-01", "张三", now)
	if err != nil {
		t.Fatal(err)
	}
	if anomaly.Resolved || interval.Version != 2 || !interval.HasOpenAnomalies() {
		t.Fatal("异常新增状态不正确")
	}
	resolved, err := interval.ResolveAnomaly("a-1", "补充取芯并确认边界", "李四", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Resolved || interval.Version != 3 || interval.HasOpenAnomalies() {
		t.Fatal("异常处置状态不正确")
	}
	if err := interval.Freeze("r-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := interval.AddAnomaly("a-2", "污染", "说明", "证据", "张三", now); err == nil {
		t.Fatal("冻结孔段仍可修改")
	}
	interval.Unfreeze("other")
	if interval.FrozenBy == "" {
		t.Fatal("其他申请不应解冻孔段")
	}
	interval.Unfreeze("r-1")
	if interval.FrozenBy != "" {
		t.Fatal("孔段未解冻")
	}
}

func TestSamplingTransitions(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	request, err := NewSamplingRequest("r-1", "c-1", []string{"i-1"}, map[string]int64{"i-1": 2}, "元素分析", "张三", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := request.Review(false, "复核员", "补充异常处置说明", now); err != nil {
		t.Fatal(err)
	}
	if request.Status != SamplingReturned || request.Version != 2 {
		t.Fatal("退回状态不正确")
	}
	if err := request.Resubmit(map[string]int64{"i-1": 3}, "元素分析及复测", "张三", now); err != nil {
		t.Fatal(err)
	}
	if request.Status != SamplingPending || request.Version != 3 {
		t.Fatal("重提状态不正确")
	}
	if err := request.Review(true, "复核员", "资料完整", now); err != nil {
		t.Fatal(err)
	}
	if err := request.Close(now); err != nil {
		t.Fatal(err)
	}
	if request.Status != SamplingClosed {
		t.Fatal("申请未关闭")
	}
}

func TestCertificateHashDetectsMutation(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	certificate, err := NewCertificate("cert-1", "c-1", "r-1", []string{"S-02", "S-01"}, "交接员", now, 1)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := VerifyCertificate(certificate)
	if err != nil || !valid {
		t.Fatalf("凭据验证失败: %v", err)
	}
	if certificate.SampleCodes[0] != "S-01" {
		t.Fatal("样品编号未规范排序")
	}
	certificate.IssuedBy = "篡改人"
	valid, err = VerifyCertificate(certificate)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("篡改后的凭据仍然有效")
	}
}
