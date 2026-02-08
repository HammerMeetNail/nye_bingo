package main

import (
	"context"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func startNotificationBackgroundJobs(
	notificationService *services.NotificationService,
	logger *logging.Logger,
) context.CancelFunc {
	if err := notificationService.CleanupOld(context.Background()); err != nil {
		logger.Warn("Notification cleanup failed", map[string]interface{}{"error": err.Error()})
	}

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	notificationService.SetAsyncContext(cleanupCtx)

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
				if err := notificationService.CleanupOld(context.Background()); err != nil {
					logger.Warn("Notification cleanup failed", map[string]interface{}{"error": err.Error()})
				}
			}
		}
	}()

	return cleanupCancel
}

func startReminderBackgroundJobs(
	reminderService *services.ReminderService,
	logger *logging.Logger,
	lookupEnv func(string) (string, bool),
) context.CancelFunc {
	if err := reminderService.CleanupOld(context.Background()); err != nil {
		logger.Warn("Reminder cleanup failed", map[string]interface{}{"error": err.Error()})
	}

	reminderCtx, reminderCancel := context.WithCancel(context.Background())
	go func() {
		interval := resolveRemindersPollInterval(logger, lookupEnv)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-reminderCtx.Done():
				return
			case <-ticker.C:
				if _, err := reminderService.RunDue(context.Background(), time.Now(), 50); err != nil {
					logger.Warn("Reminder runner failed", map[string]interface{}{"error": err.Error()})
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-reminderCtx.Done():
				return
			case <-ticker.C:
				if err := reminderService.CleanupOld(context.Background()); err != nil {
					logger.Warn("Reminder cleanup failed", map[string]interface{}{"error": err.Error()})
				}
			}
		}
	}()

	return reminderCancel
}

func resolveRemindersPollInterval(logger *logging.Logger, lookupEnv func(string) (string, bool)) time.Duration {
	interval := time.Minute
	if value, ok := lookupEnv("REMINDERS_POLL_INTERVAL"); ok && value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			logger.Warn("Invalid REMINDERS_POLL_INTERVAL; using default", map[string]interface{}{
				"value":   value,
				"default": interval.String(),
			})
		} else {
			interval = parsed
			logger.Info("Using reminders poll interval from env", map[string]interface{}{"interval": interval.String()})
		}
	}
	return interval
}
