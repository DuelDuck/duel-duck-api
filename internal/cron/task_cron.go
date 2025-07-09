package cron

import (
	"context"

	rcron "github.com/robfig/cron/v3"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"go.uber.org/zap"
)

type TaskCron struct {
	Log         *zap.Logger
	Cron        *rcron.Cron
	TaskService *service.TaskService
}

const (
	RunningDailyAt12AM = "0 0 * * *"
)

func NewTaskCron(
	l *zap.Logger,
	cron *rcron.Cron,
	taskService *service.TaskService,
) (*TaskCron, error) {
	taskCron := &TaskCron{
		Log:         l,
		Cron:        cron,
		TaskService: taskService,
	}

	_, err := taskCron.Cron.AddFunc(RunningDailyAt12AM, func() {
		taskCron.updateCreateDuelTask()
		taskCron.updateCompleteDailyStreakTask()
		taskCron.updateJoinDuelTask()
		taskCron.updateWinDuelTask()
	})
	if err != nil {
		return nil, err
	}

	return taskCron, nil
}

func (c *TaskCron) updateCreateDuelTask() {
	err := c.TaskService.UpdateCompletedTaskLimits(context.Background(), model.TaskCreateDuel)
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("task cron: successfully updated create duel task limit period")
	}
}

func (c *TaskCron) updateCompleteDailyStreakTask() {
	err := c.TaskService.UpdateCompletedTaskLimits(context.Background(), model.TaskCompletDailyStreak)
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("task cron: successfully updated complete daily streak task limit period")
	}
}

func (c *TaskCron) updateJoinDuelTask() {
	err := c.TaskService.UpdateCompletedTaskLimits(context.Background(), model.TaskJoinDuel)
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("task cron: successfully updated join a duel task limit period")
	}
}

func (c *TaskCron) updateWinDuelTask() {
	err := c.TaskService.UpdateCompletedTaskLimits(context.Background(), model.TaskWinDuel)
	if err != nil {
		LogErr(c.Log, err)
	} else {
		c.Log.Debug("task cron: successfully updated win a duel task limit period")
	}
}

func (c *TaskCron) start(_ context.Context) error {
	c.Log.Info("task cron started")
	c.Cron.Start()
	return nil
}

func (c *TaskCron) stop(_ context.Context) error {
	c.Log.Info("task cron stopped")
	c.Cron.Stop()
	return nil
}
