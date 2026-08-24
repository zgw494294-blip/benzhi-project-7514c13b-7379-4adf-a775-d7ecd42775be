package domain

func (s State) Clone() State {
	cloned := NewState()
	for id, campaign := range s.Campaigns {
		cloned.Campaigns[id] = campaign
	}
	for id, interval := range s.Intervals {
		interval.AnomalyIDs = append([]string(nil), interval.AnomalyIDs...)
		interval.Anomalies = append([]Anomaly(nil), interval.Anomalies...)
		cloned.Intervals[id] = interval
	}
	for id, request := range s.Sampling {
		request.IntervalIDs = append([]string(nil), request.IntervalIDs...)
		request.IntervalVersions = cloneVersions(request.IntervalVersions)
		cloned.Sampling[id] = request
	}
	for id, result := range s.TestResults {
		result.Measurements = make(map[string]float64, len(result.Measurements))
		for key, value := range s.TestResults[id].Measurements {
			result.Measurements[key] = value
		}
		cloned.TestResults[id] = result
	}
	for id, certificate := range s.Certificates {
		certificate.SampleCodes = append([]string(nil), certificate.SampleCodes...)
		cloned.Certificates[id] = certificate
	}
	for key, record := range s.Idempotency {
		cloned.Idempotency[key] = record
	}
	return cloned
}
