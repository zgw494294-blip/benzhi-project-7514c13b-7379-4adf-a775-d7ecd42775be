package httpapi

import (
	"net/http"

	"corelog/internal/service"
)

func (a *API) HandleCreateSamplingRequest(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.CreateSamplingCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CampaignID = campaignID
	request, err := a.service.CreateSamplingRequest(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, request)
}

func (a *API) HandleListSamplingRequests(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	requests, err := a.service.ListSamplingRequests(campaignID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, requests)
}

func (a *API) HandleGetSamplingRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	request, err := a.service.GetSamplingRequest(id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, request)
}

func (a *API) HandleReviewSamplingRequest(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.ReviewSamplingCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.RequestID = requestID
	request, err := a.service.ReviewSamplingRequest(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, request)
}

func (a *API) HandleResubmitSamplingRequest(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.ResubmitSamplingCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.RequestID = requestID
	request, err := a.service.ResubmitSamplingRequest(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, request)
}

func (a *API) HandleRecordTestResult(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.RecordTestResultCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.RequestID = requestID
	result, err := a.service.RecordTestResult(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) HandleListTestResults(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	results, err := a.service.ListTestResults(requestID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, results)
}

func (a *API) HandleGetTestResult(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "resultID")
	if !ok {
		return
	}
	result, err := a.service.GetTestResult(id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) HandleReviewTestResult(w http.ResponseWriter, r *http.Request) {
	resultID, ok := pathID(w, r, "resultID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.ReviewTestResultCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.ResultID = resultID
	result, err := a.service.ReviewTestResult(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) HandleReviewTestResultsBatch(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		Items   []service.BatchReviewItem `json:"items"`
		Results []service.BatchReviewItem `json:"results"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	items := body.Items
	if len(items) == 0 {
		items = body.Results
	}
	result, err := a.service.ReviewTestResultsBatch(requestID, items, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) HandleHandoffReadiness(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	readiness, err := a.service.HandoffReadiness(requestID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, readiness)
}

func (a *API) HandleIssueCertificate(w http.ResponseWriter, r *http.Request) {
	requestID, ok := pathID(w, r, "requestID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.IssueCertificateCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.RequestID = requestID
	certificate, err := a.service.IssueCertificate(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, certificate)
}

func (a *API) HandleGetCertificate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "certificateID")
	if !ok {
		return
	}
	certificate, err := a.service.GetCertificate(id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, certificate)
}

func (a *API) HandleVerifyCertificate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "certificateID")
	if !ok {
		return
	}
	verification, err := a.service.VerifyCertificate(id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, verification)
}
