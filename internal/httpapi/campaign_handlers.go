package httpapi

import (
	"net/http"

	"corelog/internal/service"
)

func (a *API) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) HandleSelfcheck(w http.ResponseWriter, _ *http.Request) {
	report := a.service.Selfcheck()
	status := http.StatusOK
	if !report.Passed {
		status = http.StatusServiceUnavailable
	}
	writeData(w, status, report)
}

func (a *API) HandleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.CreateCampaignCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	campaign, err := a.service.CreateCampaign(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, campaign)
}

func (a *API) HandleListCampaigns(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, a.service.ListCampaigns())
}

func (a *API) HandleGetCampaign(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	campaign, err := a.service.GetCampaign(id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, campaign)
}

func (a *API) HandleAddInterval(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.AddIntervalCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CampaignID = campaignID
	interval, err := a.service.AddInterval(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, interval)
}

func (a *API) HandleListIntervals(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	intervals, err := a.service.ListIntervals(campaignID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	progress, err := a.service.GetIntervalProgress(campaignID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"intervals": intervals, "progress": progress})
}

func (a *API) HandleAddIntervalsBatch(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.BatchIntervalsCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.CampaignID = campaignID
	result, err := a.service.AddIntervalsBatch(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) HandleListAnomalies(w http.ResponseWriter, r *http.Request) {
	campaignID, ok := pathID(w, r, "campaignID")
	if !ok {
		return
	}
	summary, err := a.service.ListAnomalies(campaignID, r.URL.Query().Get("status"), r.URL.Query().Get("kind"))
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, summary)
}

func (a *API) HandleGetInterval(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "intervalID")
	if !ok {
		return
	}
	interval, err := a.service.GetInterval(id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, interval)
}

func (a *API) HandleAddAnomaly(w http.ResponseWriter, r *http.Request) {
	intervalID, ok := pathID(w, r, "intervalID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.AddAnomalyCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.IntervalID = intervalID
	anomaly, err := a.service.AddAnomaly(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusCreated, anomaly)
}

func (a *API) HandleResolveAnomaly(w http.ResponseWriter, r *http.Request) {
	intervalID, ok := pathID(w, r, "intervalID")
	if !ok {
		return
	}
	anomalyID, ok := pathID(w, r, "anomalyID")
	if !ok {
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var command service.ResolveAnomalyCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.IntervalID, command.AnomalyID = intervalID, anomalyID
	anomaly, err := a.service.ResolveAnomaly(command, key)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeData(w, http.StatusOK, anomaly)
}
