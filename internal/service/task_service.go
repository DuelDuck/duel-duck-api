package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type TaskService struct {
	TaskRepository     *repository.TaskRepository
	UserRepository     *repository.UserRepository
	TransactionManager *repo.TransactionManager
}

func NewTaskService(
	taskRepository *repository.TaskRepository,
	userRepository *repository.UserRepository,
	transactionManager *repo.TransactionManager,
) *TaskService {
	return &TaskService{
		TaskRepository:     taskRepository,
		UserRepository:     userRepository,
		TransactionManager: transactionManager,
	}
}

func (s *TaskService) Complete(
	ctx context.Context,
	tx bun.Tx,
	completedTask *model.CompletedTask,
) error {
	task, err := s.TaskRepository.
		WithTx(tx).
		GetTaskByID(ctx, completedTask.TaskID)
	if err != nil {
		return apperrors.Internal("failed to find task by id", err)
	}

	taskForCompletion, err := s.TaskRepository.
		WithTx(tx).
		CompletedByUserID(ctx, completedTask.UserID, completedTask.TaskID)
	if err != nil {
		return apperrors.NotFound("completed task not found by id", err)
	}

	if taskForCompletion != nil {
		if task.CompletionLimit != 0 {
			if taskForCompletion.CompletionCount >= task.CompletionLimit {
				// User already reached limit of task completion
				return nil
			}
		}

		if completedTask.RewardMultiplier > 0 {
			taskForCompletion.RewardMultiplier = completedTask.RewardMultiplier
		}

		err = s.TaskRepository.WithTx(tx).UpdateCompletedTaskTimes(ctx, taskForCompletion)
		if err != nil {
			return apperrors.Internal("failed to update completed task", err)
		}

	} else {
		_, err = s.TaskRepository.WithTx(tx).Complete(ctx, completedTask)
		if err != nil {
			return err
		}

		err = s.TaskRepository.
			WithTx(tx).
			CreateCompletedTaskReward(
				ctx,
				&model.CompletedTaskReward{
					UserID: completedTask.UserID,
					TaskID: completedTask.TaskID,
				},
			)
		if err != nil {
			return apperrors.Internal("failed to create completed task reward", err)
		}
	}

	return nil
}

func (s *TaskService) CompleteSignInEmail(
	ctx context.Context,
	userID uuid.UUID,
	email mtype.Email,
	task *model.CompletedTask,
) (mtype.Balance, error) {
	user, err := s.isAbleToAttachEmail(ctx, userID, email)
	if err != nil {
		return 0, err
	}

	var userBalance mtype.Balance
	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		user.Email = email
		err = s.UserRepository.WithTx(tx).Update(ctx, user)
		if err != nil {
			return err
		}

		reward, err := s.TaskRepository.WithTx(tx).Complete(ctx, task)
		if err != nil {
			return err
		}

		userBalance, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, task.UserID, reward)
		return err
	})

	if err != nil {
		return 0, apperrors.Internal("failed to complete task", err)
	}

	return userBalance, nil
}

func (s *TaskService) GetTasksByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]*model.Task, *model.UserStats, error) {
	tasks, err := s.TaskRepository.GetTasksByUserID(ctx, userID)
	if err != nil {
		return nil, nil, apperrors.Internal("failed to get tasks", err)
	}

	s.setTaskDeadlinesAndCompletions(tasks)

	userStats, err := s.UserRepository.FindStatsByID(ctx, userID)
	if err != nil {
		return nil, nil, apperrors.Internal("failed to get user stats", err)
	}

	if userStats == nil {
		return nil, nil, apperrors.NotFound("user stats not found")
	}

	userStats.RewardTheme = model.GetRewardTheme(userStats.Level)
	userStats.NextLevelXP = s.calculateLevelXP(userStats.Level + 1)

	return tasks, userStats, nil
}

func (s *TaskService) ClaimReward(
	ctx context.Context,
	taskForReward *model.CompletedTask,
) (*model.ClaimRewardResp, error) {

	var (
		task        *model.Task
		userStats   *model.UserStats
		userBalance mtype.Balance
	)

	task, err := s.TaskRepository.GetTaskByIDAndUserID(ctx, taskForReward.TaskID, taskForReward.UserID)
	if err != nil {
		return nil, apperrors.NotFound("task not found by id", err)
	}

	// Fetch completed task
	completedTask, err := s.TaskRepository.CompletedByUserID(ctx, taskForReward.UserID, taskForReward.TaskID)
	if err != nil {
		return nil, apperrors.Internal("failed to get completed task", err)
	}
	if completedTask == nil {
		return nil, apperrors.NotFound("completed task not found by id")
	}

	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {

			if task.CompletionCount == completedTask.RewardClaimed {
				return apperrors.BadRequest("no rewards to claim")
			}

			userBalance, err = s.claimReward(ctx, tx, completedTask, task)
			if err != nil {
				return err
			}

			// Update reward taken counter
			rewardTaken, err := s.TaskRepository.WithTx(tx).UpdateRewardClaimedByID(ctx, completedTask)
			if err != nil {
				return apperrors.Internal("failed to update reward claimed counter by id", err)
			}

			task.RewardClaimed = rewardTaken

			task.Reward = int(math.Round(completedTask.RewardMultiplier * float64(task.Reward)))
			task.XP = uint32(math.Round(completedTask.RewardMultiplier * float64(task.XP)))

			s.setTaskDeadlinesAndCompletions([]*model.Task{task})

			return err

		})
	if err != nil {
		return nil, err
	}

	// Fetch user stats
	userStats, err = s.UserRepository.FindStatsByID(ctx, completedTask.UserID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user stats", err)
	}

	if userStats == nil {
		return nil, apperrors.NotFound("user stats not found")
	}

	// Set user theme based on his lvl and xp needed for the next lvl
	userStats.RewardTheme = model.GetRewardTheme(userStats.Level)
	userStats.NextLevelXP = s.calculateLevelXP(userStats.Level + 1)

	return &model.ClaimRewardResp{Task: task, UserStats: userStats, UserBalance: userBalance}, nil
}

func (s *TaskService) UpdateCompletedTaskLimits(ctx context.Context, taskID uint64) error {
	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			unclaimedRewards, err := s.TaskRepository.WithTx(tx).GetUnclaimedRewards(ctx, taskID)
			if err != nil {
				return err
			}

			task, err := s.TaskRepository.WithTx(tx).GetTaskByID(ctx, taskID)
			if err != nil {
				return err
			}

			for _, v := range unclaimedRewards {
				_, err := s.claimReward(ctx, tx, &v, task)
				if err != nil {
					return err
				}
			}

			err = s.TaskRepository.WithTx(tx).UpdateCompletedTaskLimits(ctx, taskID)
			if err != nil {
				return err
			}

			return nil
		})
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to update task limit period with id: %d", taskID), err)
	}

	return nil
}

func (s *TaskService) isAbleToAttachEmail(ctx context.Context, userID uuid.UUID, email mtype.Email) (*model.User, error) {
	userWithWantedEmail, err := s.UserRepository.FindByEmail(ctx, email)
	if err != nil {
		return nil, apperrors.Internal("failed to find user by email", err)
	}

	if userWithWantedEmail != nil {
		return nil, apperrors.AlreadyExist("received email is already in use")
	}

	currentUser, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user", err)
	}

	if currentUser.Email != "" {
		return nil, apperrors.AlreadyExist("user already attached an email")
	}

	return currentUser, nil
}

// claimReward updating user balance, lvl and xp based on the claimed rewards
func (s *TaskService) claimReward(
	ctx context.Context,
	tx bun.Tx,
	completedTask *model.CompletedTask,
	task *model.Task,
) (mtype.Balance, error) {

	userLvl, userXP, err := s.UserRepository.
		WithTx(tx).
		GetLevelAndXPByID(ctx, completedTask.UserID)
	if err != nil {
		return 0, apperrors.Internal("failed to get level and xp by id", err)
	}

	// How many rewards user has to get since his last reward claiming
	rewardCount := completedTask.CompletionCount - completedTask.RewardClaimed

	// Calculate user rewards based on reward, count of that reward and reward multiplier
	xpReward := uint32(math.Round(completedTask.RewardMultiplier * float64(task.XP*rewardCount)))
	balanceReward := int(math.Round(completedTask.RewardMultiplier * float64(task.Reward*int(rewardCount))))

	// Calculate new user lvl and xp based on task rewards
	userLvl, userXP = s.recalculateUserLevelAndXP(userLvl, userXP, xpReward)

	// Update user balance, lvl and xp
	userBalance, err := s.UserRepository.
		WithTx(tx).
		GiveRewardByID(ctx, completedTask.UserID, balanceReward, userLvl, userXP)
	if err != nil {
		return 0, apperrors.Internal("failed to give task reward by id", err)
	}

	err = s.TaskRepository.
		WithTx(tx).
		UpdateCompletedTaskRewards(
			ctx, completedTask.UserID,
			completedTask.TaskID, balanceReward, xpReward,
		)
	if err != nil {
		return 0, apperrors.Internal("failed to update completed task rewards", err)
	}

	return userBalance, nil
}

// setTaskDeadlinesAndCompletions traverses through the slice of tasks
// sets deadlines for tasks with limits periods and marks tasks as completed
func (s *TaskService) setTaskDeadlinesAndCompletions(tasks []*model.Task) {
	for _, t := range tasks {
		// If task has no limits(CompletionLimit == 0) and
		// ComplitionCount == RewardClaimed when both are bigger than 0 then
		// unlimited task is fully completed (with rewards taken)
		// Or if task has limits(ComplitionLimit > 0) and
		// all rewards are claimed(CompletionLimit == RewardClaimed)
		if (t.CompletionLimit == 0 && ((t.CompletionCount == t.RewardClaimed) && t.CompletionCount > 0)) ||
			(t.CompletionLimit > 0 && (t.CompletionLimit == t.RewardClaimed)) {

			// Set as completed
			t.Completed = true
		}

		// Set deadlines for tasks with completions
		if t.CompletionLimit > 0 && t.CompletionLimitPeriod > 0 {
			nextDeadline := uint64(time.Now().UTC().Truncate(24 * time.Hour).
				Add(time.Duration(t.CompletionLimitPeriod) * 24 * time.Hour).Unix())

			t.Deadline = &nextDeadline
		}
	}
}

// recalculateUserLevelAndXP calculates user lvl and xp based on new xp reward
func (s *TaskService) recalculateUserLevelAndXP(currentLevel, currentXP, rewardXP uint32) (uint32, uint32) {
	currentXP = currentXP + rewardXP

	xpNeeded := s.calculateLevelXP(currentLevel + 1)

	// Loop for case when reward can level up user level in several times
	for ; currentXP >= xpNeeded; xpNeeded = s.calculateLevelXP(currentLevel + 1) {
		currentXP = currentXP - xpNeeded
		currentLevel++
	}
	return currentLevel, currentXP
}

// calculateLevelXP calculates needed xp for certain level
func (s *TaskService) calculateLevelXP(level uint32) uint32 {
	xp := 1000 * math.Pow(float64(level), 1.2)
	xpRounded := uint32(math.Ceil(xp/100.0)) * 100

	return xpRounded
}
