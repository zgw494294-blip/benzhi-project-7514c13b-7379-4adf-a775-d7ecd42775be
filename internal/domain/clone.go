package domain

import "time"

func (s State) Clone() State {
	cloned := NewState()
	for id, campaign := range s.Campaigns {
		cloned.Campaigns[id] = campaign
	}
	for id, interval := range s.Intervals {
		interval.AnomalyIDs = append([]string(nil), interval.AnomalyIDs...)
		anomalies := make([]Anomaly, len(interval.Anomalies))
		for idx, anomaly := range interval.Anomalies {
			anomaly.ResolvedAt = cloneTimePointer(anomaly.ResolvedAt)
			anomalies[idx] = anomaly
		}
		interval.Anomalies = anomalies
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
		result.ReviewedAt = cloneTimePointer(result.ReviewedAt)
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

// cloneTimePointer returns an independent copy of a *time.Time so callers that
// mutate the pointed-to value through a snapshot cannot pollute the ledger's
// in-memory state. Only an explicit transaction commit may change ledger data.
func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
