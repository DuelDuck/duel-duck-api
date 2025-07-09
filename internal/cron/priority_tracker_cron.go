package cron

import (
	"context"
	rcron "github.com/robfig/cron/v3"
	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"go.uber.org/zap"
)

type PriorityTrackerCron struct {
	Log             *zap.Logger
	Cron            *rcron.Cron
	PriorityTracker *service.PriorityTracker
}

func NewPriorityTrackerCron(
	c *config.Config,
	l *zap.Logger,
	cron *rcron.Cron,
	priorityTracker *service.PriorityTracker,
) (*PriorityTrackerCron, error) {
	priorityTrackerCron := &PriorityTrackerCron{
		Log:             l,
		Cron:            cron,
		PriorityTracker: priorityTracker,
	}

	_, err := priorityTrackerCron.Cron.AddFunc(c.App.SolanaPriorityUpdateInterval, priorityTrackerCron.updatePriority)
	if err != nil {
		return nil, err
	}

	return priorityTrackerCron, nil
}

func (c *PriorityTrackerCron) updatePriority() {
	err := c.PriorityTracker.UpdatePriority()
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("priority_tracker cron: successfully updated priority")
	}
}

func (c *PriorityTrackerCron) start(_ context.Context) error {
	c.Log.Info("priority_tracker cron started")
	c.Cron.Start()
	return nil
}

func (c *PriorityTrackerCron) stop(_ context.Context) error {
	c.Log.Info("priority_tracker cron stopped")
	c.Cron.Stop()
	return nil
}
