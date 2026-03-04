package memo

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Scheduler runs memory consolidation on a daily schedule (2 AM).
type Scheduler struct {
	consolidator *Consolidator
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewScheduler creates a Scheduler that wraps a Consolidator.
func NewScheduler(consolidator *Consolidator) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		consolidator: consolidator,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins the consolidation schedule (runs at 2am daily).
func (s *Scheduler) Start() {
	go s.run()
	slog.Info("consolidation scheduler started", "time", "2:00 AM daily")
}

// Stop cancels the scheduler.
func (s *Scheduler) Stop() {
	s.cancel()
}

func (s *Scheduler) run() {
	for {
		now := time.Now()
		next2am := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())

		if now.After(next2am) {
			next2am = next2am.Add(24 * time.Hour)
		}

		duration := next2am.Sub(now)
		slog.Info("next consolidation scheduled", "time", next2am.Format("2006-01-02 15:04:05"), "in", duration.String())

		timer := time.NewTimer(duration)

		select {
		case <-timer.C:
			slog.Info("starting scheduled consolidation")

			if err := s.consolidator.ConsolidateAll(s.ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					slog.Info("consolidation cancelled due to shutdown")
					return
				}
				slog.Error("scheduled consolidation failed", "error", err)
			}

		case <-s.ctx.Done():
			timer.Stop()
			slog.Info("consolidation scheduler stopped")
			return
		}
	}
}
