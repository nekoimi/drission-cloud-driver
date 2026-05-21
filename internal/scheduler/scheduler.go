package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nekoimi/go-project-template/internal/config"
)

type Scheduler struct {
	cron   *cron.Cron
	logger *zap.Logger
	db     *gorm.DB
}

func New(cfg config.SchedulerConfig, logger *zap.Logger, db *gorm.DB) *Scheduler {
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		logger.Warn("invalid timezone, using UTC", zap.String("timezone", cfg.Timezone), zap.Error(err))
		location = time.UTC
	}

	c := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(location),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),
			cron.SkipIfStillRunning(cron.DefaultLogger),
		),
	)

	return &Scheduler{
		cron:   c,
		logger: logger,
		db:     db,
	}
}

func (s *Scheduler) AddJob(spec string, cmd cron.Job) (cron.EntryID, error) {
	return s.cron.AddJob(spec, cmd)
}

func (s *Scheduler) Start() {
	s.logger.Info("scheduler started")
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.logger.Info("scheduler stopping")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("scheduler stopped")
}
