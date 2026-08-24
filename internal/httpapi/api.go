package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"corelog/internal/domain"
	"corelog/internal/service"
)

const maxBodyBytes = 1 << 20

var resourceIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

type API struct {
	service *service.Service
	logger  *slog.Logger
}

func New(svc *service.Service, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.Default()
	}
	return &API{service: svc, logger: logger}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", a.HandleHealth)
	mux.HandleFunc("GET /v1/selfcheck", a.HandleSelfcheck)
	mux.HandleFunc("GET /v1/campaigns", a.HandleListCampaigns)
	mux.HandleFunc("POST /v1/campaigns", a.HandleCreateCampaign)
	mux.HandleFunc("GET /v1/campaigns/{campaignID}", a.HandleGetCampaign)
	mux.HandleFunc("GET /v1/campaigns/{campaignID}/intervals", a.HandleListIntervals)
	mux.HandleFunc("POST /v1/campaigns/{campaignID}/intervals", a.HandleAddInterval)
	mux.HandleFunc("POST /v1/campaigns/{campaignID}/intervals/batch", a.HandleAddIntervalsBatch)
	mux.HandleFunc("GET /v1/campaigns/{campaignID}/anomalies", a.HandleListAnomalies)
	mux.HandleFunc("GET /v1/intervals/{intervalID}", a.HandleGetInterval)
	mux.HandleFunc("POST /v1/intervals/{intervalID}/anomalies", a.HandleAddAnomaly)
	mux.HandleFunc("POST /v1/intervals/{intervalID}/anomalies/{anomalyID}/resolve", a.HandleResolveAnomaly)
	mux.HandleFunc("GET /v1/campaigns/{campaignID}/sampling-requests", a.HandleListSamplingRequests)
	mux.HandleFunc("POST /v1/campaigns/{campaignID}/sampling-requests", a.HandleCreateSamplingRequest)
	mux.HandleFunc("GET /v1/sampling-requests/{requestID}", a.HandleGetSamplingRequest)
	mux.HandleFunc("POST /v1/sampling-requests/{requestID}/review", a.HandleReviewSamplingRequest)
	mux.HandleFunc("POST /v1/sampling-requests/{requestID}/resubmit", a.HandleResubmitSamplingRequest)
	mux.HandleFunc("GET /v1/sampling-requests/{requestID}/test-results", a.HandleListTestResults)
	mux.HandleFunc("POST /v1/sampling-requests/{requestID}/test-results", a.HandleRecordTestResult)
	mux.HandleFunc("GET /v1/test-results/{resultID}", a.HandleGetTestResult)
	mux.HandleFunc("POST /v1/test-results/{resultID}/review", a.HandleReviewTestResult)
	mux.HandleFunc("POST /v1/sampling-requests/{requestID}/test-results/review-batch", a.HandleReviewTestResultsBatch)
	mux.HandleFunc("POST /v1/sampling-requests/{requestID}/test-results/review", a.HandleReviewTestResultsBatch)
	mux.HandleFunc("GET /v1/sampling-requests/{requestID}/handoff-readiness", a.HandleHandoffReadiness)
	mux.HandleFunc("GET /v1/sampling-requests/{requestID}/readiness", a.HandleHandoffReadiness)
	mux.HandleFunc("POST /v1/sampling-requests/{requestID}/certificates", a.HandleIssueCertificate)
	mux.HandleFunc("GET /v1/certificates/{certificateID}", a.HandleGetCertificate)
	mux.HandleFunc("GET /v1/certificates/{certificateID}/verify", a.HandleVerifyCertificate)
	return a.recoverPanic(a.accessLog(mux))
}

func (a *API) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (a *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("handler panic", "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误", "")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeData(w http.ResponseWriter, status int, value any) {
	writeJSON(w, status, responseEnvelope{Data: value})
}

func writeError(w http.ResponseWriter, status int, code, message, field string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message, Field: field}})
}

func handleServiceError(w http.ResponseWriter, err error) {
	var domainError *domain.Error
	if !errors.As(err, &domainError) {
		writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误", "")
		return
	}
	status := http.StatusBadRequest
	switch domainError.Code {
	case "not_found":
		status = http.StatusNotFound
	case "conflict", "invalid_state":
		status = http.StatusConflict
	}
	writeError(w, status, domainError.Code, domainError.Message, domainError.Field)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type 必须为 application/json", "Content-Type")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		message := "请求体不是有效 JSON"
		if errors.Is(err, io.EOF) {
			message = "请求体不能为空"
		}
		writeError(w, http.StatusBadRequest, "invalid_json", message, "body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求体只能包含一个 JSON 对象", "body")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := strings.TrimSpace(r.PathValue(name))
	if !resourceIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "validation_error", "路径标识格式无效", name)
		return "", false
	}
	return id, true
}

func idempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "缺少 Idempotency-Key", "Idempotency-Key")
		return "", false
	}
	if len(key) > 128 {
		writeError(w, http.StatusBadRequest, "validation_error", "Idempotency-Key 长度不能超过 128", "Idempotency-Key")
		return "", false
	}
	return key, true
}
