package scheduler

import (
	"go.uber.org/zap"

	"github.com/nekoimi/go-project-template/internal/scheduler/jobs"
)

func (s *Scheduler) RegisterJobs() {
	// Register all cron jobs here
	// Example: every 5 minutes
	if _, err := s.AddJob("0 */5 * * * *", jobs.NewExampleJob(s.logger)); err != nil {
		s.logger.Error("failed to register example job", zap.Error(err))
	}
}
