package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"corelog/internal/repository"
	"corelog/internal/service"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repo, err := repository.New(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(service.New(repo), nil).Routes()
}

func request(t *testing.T, handler http.Handler, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &encoded)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestCampaignHTTPAndIdempotency(t *testing.T) {
	handler := testHandler(t)
	body := map[string]any{"name": "北区钻探", "site": "3号平台", "coordinateReference": "CGCS2000", "coordinator": "王工"}
	first := request(t, handler, http.MethodPost, "/v1/campaigns", "http-key", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("创建状态=%d body=%s", first.Code, first.Body.String())
	}
	second := request(t, handler, http.MethodPost, "/v1/campaigns", "http-key", body)
	if second.Code != http.StatusCreated {
		t.Fatalf("重放状态=%d", second.Code)
	}
	var firstData, secondData struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstData); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondData); err != nil {
		t.Fatal(err)
	}
	if firstData.Data.ID == "" || firstData.Data.ID != secondData.Data.ID {
		t.Fatal("HTTP 幂等响应不一致")
	}
	get := request(t, handler, http.MethodGet, "/v1/campaigns/"+firstData.Data.ID, "", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("查询状态=%d", get.Code)
	}
}

func TestHTTPValidationAndSelfcheck(t *testing.T) {
	handler := testHandler(t)
	body := map[string]any{"name": "任务", "site": "平台", "coordinateReference": "CGCS2000", "coordinator": "王工"}
	missingKey := request(t, handler, http.MethodPost, "/v1/campaigns", "", body)
	if missingKey.Code != http.StatusBadRequest {
		t.Fatalf("缺少幂等键状态=%d", missingKey.Code)
	}
	unknown := body
	unknown["unexpected"] = true
	invalid := request(t, handler, http.MethodPost, "/v1/campaigns", "key", unknown)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("未知字段状态=%d", invalid.Code)
	}
	check := request(t, handler, http.MethodGet, "/v1/selfcheck", "", nil)
	if check.Code != http.StatusOK {
		t.Fatalf("空账本自检状态=%d body=%s", check.Code, check.Body.String())
	}
	badPath := request(t, handler, http.MethodGet, "/v1/campaigns/%25bad", "", nil)
	if badPath.Code != http.StatusBadRequest {
		t.Fatalf("非法路径状态=%d", badPath.Code)
	}
}
