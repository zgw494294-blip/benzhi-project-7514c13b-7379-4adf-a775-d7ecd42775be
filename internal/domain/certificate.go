package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type CertificatePayload struct {
	ID                string    `json:"id"`
	CampaignID        string    `json:"campaignID"`
	SamplingRequestID string    `json:"samplingRequestID"`
	SampleCodes       []string  `json:"sampleCodes"`
	IssuedBy          string    `json:"issuedBy"`
	IssuedAt          time.Time `json:"issuedAt"`
	Sequence          int64     `json:"sequence"`
	SchemaVersion     int       `json:"schemaVersion"`
}

func NewCertificate(id, campaignID, requestID string, sampleCodes []string, issuedBy string, issuedAt time.Time, sequence int64) (CustodyCertificate, error) {
	for field, value := range map[string]string{"id": id, "campaignID": campaignID, "samplingRequestID": requestID, "issuedBy": issuedBy} {
		if strings.TrimSpace(value) == "" {
			return CustodyCertificate{}, Required(field)
		}
	}
	if len(sampleCodes) == 0 {
		return CustodyCertificate{}, Invalid("sampleCodes", "至少包含一个样品编号")
	}
	if sequence < 1 {
		return CustodyCertificate{}, Invalid("sequence", "必须大于 0")
	}
	codes := append([]string(nil), sampleCodes...)
	sort.Strings(codes)
	for idx, code := range codes {
		if strings.TrimSpace(code) == "" {
			return CustodyCertificate{}, Invalid("sampleCodes", "不能包含空编号")
		}
		codes[idx] = strings.TrimSpace(code)
		if idx > 0 && codes[idx] == codes[idx-1] {
			return CustodyCertificate{}, Invalid("sampleCodes", "不能包含重复编号")
		}
	}
	certificate := CustodyCertificate{
		ID: id, CampaignID: campaignID, SamplingRequestID: requestID, SampleCodes: codes,
		IssuedBy: strings.TrimSpace(issuedBy), IssuedAt: issuedAt.UTC(), Sequence: sequence, SchemaVersion: SchemaVersion,
	}
	hash, err := CertificateHash(certificate)
	if err != nil {
		return CustodyCertificate{}, err
	}
	certificate.PayloadHash = hash
	return certificate, nil
}

func CertificateHash(c CustodyCertificate) (string, error) {
	payload := CertificatePayload{
		ID: c.ID, CampaignID: c.CampaignID, SamplingRequestID: c.SamplingRequestID,
		SampleCodes: append([]string(nil), c.SampleCodes...), IssuedBy: c.IssuedBy,
		IssuedAt: c.IssuedAt.UTC(), Sequence: c.Sequence, SchemaVersion: c.SchemaVersion,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyCertificate(c CustodyCertificate) (bool, error) {
	if c.SchemaVersion != SchemaVersion {
		return false, Invalid("schemaVersion", "凭据版本不受支持")
	}
	expected, err := CertificateHash(c)
	if err != nil {
		return false, err
	}
	return expected == c.PayloadHash, nil
}
