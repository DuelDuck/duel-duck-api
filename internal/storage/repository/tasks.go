package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type TaskRepository struct {
	DB    repository.DB
	tasks map[uint64]model.Task
}

func NewTaskRepository(
	db repository.DB,
) (*TaskRepository, error) {
	taskRepository := &TaskRepository{DB: db}

	err := taskRepository.initTasks()
	if err != nil {
		return nil, err
	}

	if taskRepository.tasks == nil {
		return nil, errors.New("tasks is empty")
	}

	return taskRepository, nil
}

func (r *TaskRepository) WithTx(tx bun.Tx) *TaskRepository {
	return &TaskRepository{
		DB:    r.DB.WithTx(tx),
		tasks: r.tasks,
	}
}

func (r *TaskRepository) initTasks() error {
	var taskList []model.Task

	err := r.DB.NewSelect().
		Model((*model.Task)(nil)).
		ExcludeColumn("completed").
		ExcludeColumn("completion_count").
		ExcludeColumn("reward_claimed").
		Scan(context.Background(), &taskList)
	if err != nil {
		return err
	}

	r.tasks = make(map[uint64]model.Task, len(taskList))
	for _, t := range taskList {
		r.tasks[t.ID] = t
	}

	return nil
}

func (r *TaskRepository) Complete(
	ctx context.Context,
	completed *model.CompletedTask,
) (int, error) {
	_, err := r.DB.NewInsert().
		Model(completed).
		Exec(ctx)
	if err != nil {
		return 0, err
	}

	task, ok := r.tasks[completed.TaskID]
	if !ok {
		return 0, apperrors.Internal("task not found")
	}

	return task.Reward, nil
}

func (r *TaskRepository) UpdateCompletedTaskTimes(
	ctx context.Context,
	completed *model.CompletedTask,
) error {
	_, err := r.DB.NewUpdate().
		Model((*model.CompletedTask)(nil)).
		Set("overall_count = overall_count + 1").
		Set("completion_count = completion_count + 1").
		Set("reward_multiplier = ?", completed.RewardMultiplier).
		Set("updated_at = current_timestamp").
		Where("user_id = ?", completed.UserID).
		Where("task_id = ?", completed.TaskID).
		Exec(ctx)
	if err != nil {
		return err
	}

	_, ok := r.tasks[completed.TaskID]
	if !ok {
		return apperrors.Internal("task not found")
	}

	return nil
}

func (r *TaskRepository) UpdateCompletedTaskRewards(
	ctx context.Context,
	userID uuid.UUID,
	taskID uint64,
	reward int,
	xp uint32,
) error {
	_, err := r.DB.NewUpdate().
		Model((*model.CompletedTaskReward)(nil)).
		Set("overall_reward = overall_reward + ?", reward).
		Set("overall_xp = overall_xp + ?", xp).
		Set("updated_at = current_timestamp").
		Where("user_id = ?", userID).
		Where("task_id = ?", taskID).
		Exec(ctx)
	if err != nil {
		return err
	}

	_, ok := r.tasks[taskID]
	if !ok {
		return apperrors.Internal("task not found")
	}

	return nil
}

func (r *TaskRepository) UpdateRewardClaimedByID(
	ctx context.Context, completed *model.CompletedTask,
) (uint32, error) {
	var rewardTaken uint32

	err := r.DB.NewUpdate().
		Model((*model.CompletedTask)(nil)).
		Set("reward_claimed = completion_count").
		Where("user_id = ?", completed.UserID).
		Where("task_id = ?", completed.TaskID).
		Returning("reward_claimed").
		Scan(ctx, &rewardTaken)
	if err != nil {
		return 0, err
	}

	return rewardTaken, nil
}

func (r *TaskRepository) GetTasksByUserID(ctx context.Context, userID uuid.UUID) ([]*model.Task, error) {
	var userTasks []*model.Task

	err := r.DB.NewSelect().
		Model((*model.Task)(nil)).
		Column(
			"id",
			"name",
			"description",
			"reward",
			"currency",
			"xp",
			"completion_limit",
			"completion_limit_period",
			"auto_completion",
			"link",
			"t.created_at",
			"c.reward_claimed",
		).
		ColumnExpr("c.completion_count AS completion_count").
		Join("LEFT JOIN completed_tasks AS c").
		JoinOn("t.id = c.task_id").
		JoinOn("c.user_id = ?", userID).
		OrderExpr("t.id ASC").
		Scan(ctx, &userTasks)
	if err != nil {
		return nil, err
	}

	return userTasks, nil
}

func (r *TaskRepository) GetTaskByID(ctx context.Context, id uint64) (*model.Task, error) {
	var task = new(model.Task)

	err := r.DB.NewSelect().
		Model(task).
		Column(
			"id",
			"name",
			"description",
			"reward",
			"currency",
			"xp",
			"completion_limit",
			"completion_limit_period",
			"auto_completion",
			"link",
			"t.created_at",
		).
		Where("t.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (r *TaskRepository) GetTaskByIDAndUserID(
	ctx context.Context,
	taskID uint64,
	userID uuid.UUID,
) (*model.Task, error) {
	var task = new(model.Task)

	err := r.DB.NewSelect().
		Model(task).
		Column(
			"id",
			"name",
			"description",
			"reward",
			"currency",
			"xp",
			"completion_limit",
			"completion_limit_period",
			"auto_completion",
			"link",
			"t.created_at",
			"c.reward_claimed",
		).
		ColumnExpr("c.completion_count AS completion_count").
		Join("LEFT JOIN completed_tasks AS c").
		JoinOn("t.id = c.task_id").
		JoinOn("c.user_id = ?", userID).
		Where("t.id = ?", taskID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return task, nil
}

func (r *TaskRepository) FindCompletedByUserID(
	ctx context.Context,
	userID uuid.UUID,
	taskID uint64,
) (*model.CompletedTask, error) {
	var completedTask = new(model.CompletedTask)

	err := r.DB.NewSelect().
		Model(completedTask).
		Where("user_id = ?", userID).
		Where("task_id = ?", taskID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return completedTask, nil
}

func (r *TaskRepository) GetUnclaimedRewards(
    ctx context.Context,
    id uint64,
) ([]model.CompletedTask, error) {

    unclaimedRewards := make([]model.CompletedTask, 0)

    err := r.DB.NewSelect().
        Model(&unclaimedRewards).
        Where("task_id = ?", id).
        Where("reward_claimed < completion_count").
        Scan(ctx)
    if err != nil {
         return nil, err
    }

    return unclaimedRewards, nil
}

func (r *TaskRepository) UpdateCompletedTaskLimits(
	ctx context.Context,
	id uint64) error {

	_, err := r.DB.NewUpdate().
		Model((*model.CompletedTask)(nil)).
		TableExpr("tasks AS t").
		Set("completion_count = 0").
		Set("reward_claimed = 0").
		Where("c.task_id = t.id").
		Where("c.task_id = ?", id).
		Exec(ctx)

	return err
}

func (r *TaskRepository) CreateCompletedTaskReward(
	ctx context.Context,
	completedTaskReward *model.CompletedTaskReward,
) error {

	_, err := r.DB.NewInsert().
		Model(completedTaskReward).
		Exec(ctx)

	return err
}
