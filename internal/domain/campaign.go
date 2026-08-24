package domain

import (
	"strings"
	"time"
)

func NewCampaign(id, name, site, coordinateReference, coordinator string, now time.Time) (DrillingCampaign, error) {
	fields := map[string]string{
		"id": id, "name": name, "site": site,
		"coordinateReference": coordinateReference, "coordinator": coordinator,
	}
	for field, value := range fields {
		if strings.TrimSpace(value) == "" {
			return DrillingCampaign{}, Required(field)
		}
	}
	return DrillingCampaign{
		ID: strings.TrimSpace(id), Name: strings.TrimSpace(name), Site: strings.TrimSpace(site),
		CoordinateReference: strings.TrimSpace(coordinateReference), Coordinator: strings.TrimSpace(coordinator),
		Status: CampaignActive, Version: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}, nil
}

func (c *DrillingCampaign) EnsureActive() error {
	if c.Status != CampaignActive {
		return StateConflict("钻探任务已完成样品交接，不能继续修改")
	}
	return nil
}

func (c *DrillingCampaign) CheckVersion(expected int64) error {
	if expected < 1 {
		return Invalid("expectedVersion", "必须大于 0")
	}
	if c.Version != expected {
		return Conflict("钻探任务版本已变化")
	}
	return nil
}

func (c *DrillingCampaign) Touch(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC()
}

func (c *DrillingCampaign) MarkCustodyIssued(now time.Time) error {
	if err := c.EnsureActive(); err != nil {
		return err
	}
	c.Status = CampaignCustodyIssued
	c.Touch(now)
	return nil
}
