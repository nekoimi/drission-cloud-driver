package driver

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/config"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
)

type offlineSyncer struct {
	store      offline.Store
	registry   *drivers.Registry
	browserMgr *browser.Manager
	config     config.OfflineSyncConfig
	logger     *zap.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newOfflineSyncer(
	store offline.Store,
	registry *drivers.Registry,
	browserMgr *browser.Manager,
	cfg config.OfflineSyncConfig,
	logger *zap.Logger,
) *offlineSyncer {
	return &offlineSyncer{
		store:      store,
		registry:   registry,
		browserMgr: browserMgr,
		config:     cfg,
		logger:     logger,
	}
}

func (s *offlineSyncer) Start(parent context.Context) {
	if s == nil || !s.config.Enabled || s.store == nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()
}

func (s *offlineSyncer) Stop(ctx context.Context) error {
	if s == nil || s.cancel == nil {
		return nil
	}
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *offlineSyncer) run(ctx context.Context) {
	interval := time.Duration(s.config.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}

	s.syncOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncOnce(ctx)
		}
	}
}

type offlineSyncGroup struct {
	platform  string
	profileID string
	records   []offline.OfflineTaskRecord
}

func (s *offlineSyncer) syncOnce(ctx context.Context) {
	if locker, ok := s.store.(offline.SyncCycleLocker); ok {
		locked, err := locker.WithSyncCycleLock(ctx, func() error {
			s.syncOnceLocked(ctx)
			return nil
		})
		if err != nil {
			s.logger.Warn("offline sync lock failed", zap.Error(err))
		} else if !locked {
			s.logger.Debug("offline sync skipped because another replica holds the lock")
		}
		return
	}
	s.syncOnceLocked(ctx)
}

func (s *offlineSyncer) syncOnceLocked(ctx context.Context) {
	records, err := s.store.List("", "")
	if err != nil {
		s.logger.Warn("list offline tasks for sync failed", zap.Error(err))
		return
	}

	groups := make(map[string]*offlineSyncGroup)
	for _, record := range records {
		if !needsRemoteSync(record) {
			continue
		}
		key := record.Platform + "\x00" + record.ProfileID
		group := groups[key]
		if group == nil {
			group = &offlineSyncGroup{platform: record.Platform, profileID: record.ProfileID}
			groups[key] = group
		}
		group.records = append(group.records, record)
	}

	for _, group := range groups {
		if ctx.Err() != nil {
			return
		}
		s.syncGroup(ctx, group)
	}

	if s.config.CleanupCompleted {
		s.cleanupCompleted(ctx)
	}
}

func needsRemoteSync(record offline.OfflineTaskRecord) bool {
	if !record.RemoteCleanedAt.IsZero() {
		return false
	}
	if record.SyncError != "" && !retryDue(record.UpdatedAt, record.SyncAttempts) {
		return false
	}
	switch record.Task.Status {
	case drivers.TaskPending, drivers.TaskRunning, drivers.TaskUnknown:
		return true
	default:
		return false
	}
}

func retryDue(lastAttempt time.Time, attempts int) bool {
	if attempts <= 0 || lastAttempt.IsZero() {
		return true
	}
	delay := 30 * time.Second
	for i := 1; i < attempts && delay < 10*time.Minute; i++ {
		delay *= 2
	}
	if delay > 10*time.Minute {
		delay = 10 * time.Minute
	}
	return time.Since(lastAttempt) >= delay
}

func (s *offlineSyncer) syncGroup(ctx context.Context, group *offlineSyncGroup) {
	driver, err := s.registry.Get(group.platform, s.browserMgr)
	if err != nil {
		s.markGroupSyncError(group.records, err)
		return
	}

	remoteList, err := driver.ListOfflineTasks(ctx, group.profileID)
	if err != nil {
		s.markGroupSyncError(group.records, err)
		return
	}

	remoteByID := make(map[string]drivers.OfflineTask, len(remoteList.Items))
	for _, task := range remoteList.Items {
		remoteByID[strings.ToLower(task.ProviderTaskID)] = task
	}

	now := time.Now()
	for _, record := range group.records {
		providerID := record.Task.ProviderTaskID
		if providerID == "" {
			providerID = drivers.ProviderTaskID(record.Platform, record.Task.TaskID)
		}
		task, ok := remoteByID[strings.ToLower(providerID)]
		if !ok {
			record.SyncAttempts++
			record.SyncError = "provider task not found"
			if record.SyncAttempts >= 3 {
				record.Task.Status = drivers.TaskUnknown
			}
			if err := s.store.Update(record); err != nil {
				s.logger.Warn("save missing offline task sync state failed",
					zap.String("task_id", record.Task.TaskID),
					zap.Error(err),
				)
			}
			continue
		}

		previousStatus := record.Task.Status
		normalizeStoredOfflineTask(&record, &task)
		if record.Task.Status == drivers.TaskCompleted && task.Status != drivers.TaskCompleted {
			task.Status = drivers.TaskCompleted
		}
		record.Task = task
		record.LastSyncedAt = now
		record.SyncAttempts = 0
		record.SyncError = ""
		if task.Status == drivers.TaskCompleted && record.CompletedAt.IsZero() {
			record.CompletedAt = now
		}
		if err := s.store.Update(record); err != nil {
			s.logger.Warn("update synchronized offline task failed",
				zap.String("task_id", task.TaskID),
				zap.Error(err),
			)
		} else if previousStatus != task.Status {
			s.logger.Info("offline task status synchronized",
				zap.String("task_id", task.TaskID),
				zap.String("from", string(previousStatus)),
				zap.String("to", string(task.Status)),
				zap.Int("provider_status", task.ProviderStatus),
				zap.Float64("progress", task.Progress),
			)
		}
	}
}

func (s *offlineSyncer) markGroupSyncError(records []offline.OfflineTaskRecord, syncErr error) {
	for _, record := range records {
		record.SyncAttempts++
		record.SyncError = syncErr.Error()
		if err := s.store.Update(record); err != nil {
			s.logger.Warn("save offline task sync error failed",
				zap.String("task_id", record.Task.TaskID),
				zap.Error(err),
			)
		}
	}
	s.logger.Warn("offline task profile sync failed", zap.Error(syncErr))
}

func (s *offlineSyncer) cleanupCompleted(ctx context.Context) {
	records, err := s.store.List("", "")
	if err != nil {
		s.logger.Warn("list completed offline tasks for cleanup failed", zap.Error(err))
		return
	}

	grace := time.Duration(s.config.CleanupGraceSeconds) * time.Second
	if grace < 0 {
		grace = 0
	}
	now := time.Now()
	for _, record := range records {
		if ctx.Err() != nil {
			return
		}
		if record.Task.Status != drivers.TaskCompleted || !record.RemoteCleanedAt.IsZero() {
			continue
		}
		if record.CleanupError != "" && !retryDue(record.UpdatedAt, record.CleanupAttempts) {
			continue
		}
		if record.CompletedAt.IsZero() {
			record.CompletedAt = now
			if err := s.store.Update(record); err != nil {
				s.logger.Warn("initialize offline task completion time failed",
					zap.String("task_id", record.Task.TaskID),
					zap.Error(err),
				)
				continue
			}
		}
		if now.Sub(record.CompletedAt) < grace {
			continue
		}

		driver, err := s.registry.Get(record.Platform, s.browserMgr)
		if err == nil {
			err = driver.RemoveOfflineTask(ctx, record.ProfileID, record.Task.TaskID)
		}
		if err != nil {
			record.CleanupAttempts++
			record.CleanupError = err.Error()
			if updateErr := s.store.Update(record); updateErr != nil {
				s.logger.Warn("save offline task cleanup error failed",
					zap.String("task_id", record.Task.TaskID),
					zap.Error(updateErr),
				)
			}
			continue
		}

		record.RemoteCleanedAt = now
		record.CleanupAttempts = 0
		record.CleanupError = ""
		if err := s.store.Update(record); err != nil {
			s.logger.Warn("mark remote offline task cleaned failed",
				zap.String("task_id", record.Task.TaskID),
				zap.Error(err),
			)
			continue
		}
		s.logger.Info("cleaned completed provider offline task",
			zap.String("task_id", record.Task.TaskID),
			zap.String("platform", record.Platform),
			zap.String("profile_id", record.ProfileID),
		)
	}
}
