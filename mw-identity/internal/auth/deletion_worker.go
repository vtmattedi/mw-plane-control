package auth

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"
)

// RunDeletionWorker blocks until shutdown and waits for every bounded worker.
func (s *Service) RunDeletionWorker(ctx context.Context) {
	if !s.durableDeletion {
		return
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timer := time.NewTicker(time.Second)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					if _, err := s.ProcessDeletionDelivery(ctx); err != nil && ctx.Err() == nil {
						slog.Error("deletion worker storage failure")
					}
				}
			}
		}()
	}
	wg.Wait()
}

// ProcessDeletionDelivery also supports isolated deterministic runtime checks.
func (s *Service) ProcessDeletionDelivery(ctx context.Context) (bool, error) {
	store, err := s.DurableDeletionStore()
	if err != nil {
		return false, err
	}
	lease, err := NewOpaqueToken("lease_")
	if err != nil {
		return false, err
	}
	now := s.now().UTC()
	d, err := store.ClaimDeletionDelivery(ctx, now, lease)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, store.FinalizeReadyDeletions(ctx, now)
		}
		return false, err
	}
	status, category := "pending", "callback_unavailable"
	if d.ErrorCode != "" {
		status, category = "unresolved", d.ErrorCode
	} else if d.Attempts > 1 && now.Sub(d.RetryStartedAt) > 7*24*time.Hour {
		status, category = "failed", "retry_horizon"
	} else {
		status, category = s.deliverDurableDeletion(ctx, d, now)
	}
	delay := time.Second * time.Duration(1<<min(d.Attempts, 12))
	delay = min(delay, time.Hour)
	delay = min(time.Hour, delay+time.Duration(rand.Int64N(int64(delay/4)+1)))
	// Persist using a separate bounded context: shutdown leaves a retryable lease if persistence fails.
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err = store.FinishDeletionDelivery(persist, d, status, category, now.Add(delay)); err != nil {
		return true, err
	}
	return true, store.FinalizeDurableDeletion(persist, d.OperationID, now)
}
