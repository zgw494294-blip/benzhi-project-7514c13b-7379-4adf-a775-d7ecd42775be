package batch_context_cancel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"corelog/internal/domain"
	"corelog/internal/httpapi"
	"corelog/internal/repository"
	"corelog/internal/service"
)

type observedContext struct {
	done     chan struct{}
	observed chan struct{}
	once     sync.Once
	mu       sync.Mutex
	err      error
}

func newObservedContext() *observedContext {
	return &observedContext{done: make(chan struct{}), observed: make(chan struct{})}
}

func (c *observedContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *observedContext) Done() <-chan struct{}       { return c.done }
func (c *observedContext) Value(any) any               { return nil }

func (c *observedContext) Err() error {
	c.once.Do(func() { close(c.observed) })
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *observedContext) cancel() {
	c.mu.Lock()
	c.err = context.Canceled
	c.mu.Unlock()
	close(c.done)
}

func TestCanceledBatchWaitingForWriteLockDoesNotCommit(t *testing.T) {
	repo, err := repository.New(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	nextID := 0
	svc := service.NewWithDependencies(repo, func() time.Time {
		now = now.Add(time.Second)
		return now
	}, func(prefix string) string {
		nextID++
		return prefix + "-context"
	})
	campaign, err := svc.CreateCampaign(service.CreateCampaignCommand{
		Name: "取消复现任务", Site: "北区平台", CoordinateReference: "CGCS2000", Coordinator: "复核员",
	}, "create-context-campaign")
	if err != nil {
		t.Fatal(err)
	}

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- repo.Transact(func(_ *domain.State) error {
			close(lockHeld)
			<-releaseLock
			return nil
		})
	}()
	<-lockHeld

	body, err := json.Marshal(map[string]any{
		"expectedVersion": campaign.Version,
		"intervals": []map[string]any{{
			"boreholeCode": "ZK-CANCEL", "depthStart": 0, "depthEnd": 8,
			"lithology": "花岗岩", "recoveryRate": 95, "condition": "完整",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	requestContext := newObservedContext()
	req := httptest.NewRequest(http.MethodPost, "/v1/campaigns/"+campaign.ID+"/intervals/batch", bytes.NewReader(body)).WithContext(requestContext)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "canceled-batch")
	recorder := httptest.NewRecorder()
	requestDone := make(chan struct{})
	go func() {
		httpapi.New(svc, nil).Routes().ServeHTTP(recorder, req)
		close(requestDone)
	}()

	<-requestContext.observed
	requestContext.cancel()
	close(releaseLock)
	if err := <-holderDone; err != nil {
		t.Fatal(err)
	}
	<-requestDone

	state := repo.Snapshot()
	if recorder.Code == http.StatusCreated || len(state.Intervals) != 0 {
		t.Fatalf("取消的批量请求仍返回 status=%d 并提交 %d 个孔段", recorder.Code, len(state.Intervals))
	}
}
