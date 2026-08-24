package domain

import "time"

const SchemaVersion = 1

type CampaignStatus string

const (
	CampaignActive        CampaignStatus = "active"
	CampaignCustodyIssued CampaignStatus = "custody_issued"
)

type DrillingCampaign struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Site                string         `json:"site"`
	CoordinateReference string         `json:"coordinateReference"`
	Coordinator         string         `json:"coordinator"`
	Status              CampaignStatus `json:"status"`
	Version             int64          `json:"version"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

type Anomaly struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	Evidence    string     `json:"evidence"`
	ReportedBy  string     `json:"reportedBy"`
	ReportedAt  time.Time  `json:"reportedAt"`
	Resolved    bool       `json:"resolved"`
	Resolution  string     `json:"resolution,omitempty"`
	ResolvedBy  string     `json:"resolvedBy,omitempty"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
}

type CoreInterval struct {
	ID            string    `json:"id"`
	CampaignID    string    `json:"campaignID"`
	BoreholeCode  string    `json:"boreholeCode"`
	DepthStart    float64   `json:"depthStart"`
	DepthEnd      float64   `json:"depthEnd"`
	Lithology     string    `json:"lithology"`
	RecoveryRate  float64   `json:"recoveryRate"`
	Condition     string    `json:"condition"`
	AnomalyIDs    []string  `json:"anomalyIDs"`
	Anomalies     []Anomaly `json:"anomalies"`
	FrozenBy      string    `json:"frozenBy,omitempty"`
	FrozenVersion int64     `json:"frozenVersion,omitempty"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type SamplingStatus string

const (
	SamplingPending  SamplingStatus = "pending_review"
	SamplingReturned SamplingStatus = "returned"
	SamplingApproved SamplingStatus = "approved"
	SamplingClosed   SamplingStatus = "certificate_issued"
)

type SamplingRequest struct {
	ID               string           `json:"id"`
	CampaignID       string           `json:"campaignID"`
	IntervalIDs      []string         `json:"intervalIDs"`
	IntervalVersions map[string]int64 `json:"intervalVersions"`
	Purpose          string           `json:"purpose"`
	RequestedBy      string           `json:"requestedBy"`
	Status           SamplingStatus   `json:"status"`
	Reviewer         string           `json:"reviewer,omitempty"`
	ReviewNote       string           `json:"reviewNote,omitempty"`
	Version          int64            `json:"version"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewReturned ReviewStatus = "returned"
	ReviewApproved ReviewStatus = "approved"
)

type TestResult struct {
	ID                string             `json:"id"`
	SamplingRequestID string             `json:"samplingRequestID"`
	SampleCode        string             `json:"sampleCode"`
	Method            string             `json:"method"`
	Measurements      map[string]float64 `json:"measurements"`
	Unit              string             `json:"unit"`
	PerformedBy       string             `json:"performedBy"`
	ReviewStatus      ReviewStatus       `json:"reviewStatus"`
	ReviewNote        string             `json:"reviewNote,omitempty"`
	ReviewedBy        string             `json:"reviewedBy,omitempty"`
	Version           int64              `json:"version"`
	RecordedAt        time.Time          `json:"recordedAt"`
	ReviewedAt        *time.Time         `json:"reviewedAt,omitempty"`
}

type CustodyCertificate struct {
	ID                string    `json:"id"`
	CampaignID        string    `json:"campaignID"`
	SamplingRequestID string    `json:"samplingRequestID"`
	SampleCodes       []string  `json:"sampleCodes"`
	IssuedBy          string    `json:"issuedBy"`
	IssuedAt          time.Time `json:"issuedAt"`
	PayloadHash       string    `json:"payloadHash"`
	Sequence          int64     `json:"sequence"`
	SchemaVersion     int       `json:"schemaVersion"`
}

type State struct {
	Campaigns    map[string]DrillingCampaign   `json:"campaigns"`
	Intervals    map[string]CoreInterval       `json:"intervals"`
	Sampling     map[string]SamplingRequest    `json:"samplingRequests"`
	TestResults  map[string]TestResult         `json:"testResults"`
	Certificates map[string]CustodyCertificate `json:"certificates"`
	Idempotency  map[string]IdempotencyRecord  `json:"idempotency"`
}

type IdempotencyRecord struct {
	Operation   string    `json:"operation"`
	RequestHash string    `json:"requestHash"`
	ResourceID  string    `json:"resourceID"`
	RecordedAt  time.Time `json:"recordedAt"`
}

type BoreholeProgress struct {
	BoreholeCode     string  `json:"boreholeCode"`
	IntervalCount    int     `json:"intervalCount"`
	DepthStart       float64 `json:"depthStart"`
	DepthEnd         float64 `json:"depthEnd"`
	LatestIntervalID string  `json:"latestIntervalID"`
	LatestVersion    int64   `json:"latestVersion"`
	ContinuityStatus string  `json:"continuityStatus"`
}

type AnomalyEvidence struct {
	ID           string    `json:"id"`
	BoreholeCode string    `json:"boreholeCode"`
	IntervalID   string    `json:"intervalID"`
	DepthStart   float64   `json:"depthStart"`
	DepthEnd     float64   `json:"depthEnd"`
	Kind         string    `json:"kind"`
	Description  string    `json:"description"`
	Evidence     string    `json:"evidence"`
	ReportedBy   string    `json:"reportedBy"`
	ReportedAt   time.Time `json:"reportedAt"`
	Status       string    `json:"status"`
}

type AnomalySummary struct {
	Items             []AnomalyEvidence `json:"items"`
	Total             int               `json:"total"`
	Open              int               `json:"open"`
	Resolved          int               `json:"resolved"`
	BlockingBoreholes []string          `json:"blockingBoreholes"`
	BlockingIntervals []string          `json:"blockingIntervals"`
}

type HandoffReadiness struct {
	SamplingRequestID string   `json:"samplingRequestID"`
	Total             int      `json:"total"`
	Pending           int      `json:"pending"`
	Returned          int      `json:"returned"`
	Approved          int      `json:"approved"`
	CanIssue          bool     `json:"canIssue"`
	BlockingResults   []string `json:"blockingResults"`
}

func NewState() State {
	return State{
		Campaigns: make(map[string]DrillingCampaign), Intervals: make(map[string]CoreInterval),
		Sampling: make(map[string]SamplingRequest), TestResults: make(map[string]TestResult),
		Certificates: make(map[string]CustodyCertificate), Idempotency: make(map[string]IdempotencyRecord),
	}
}
