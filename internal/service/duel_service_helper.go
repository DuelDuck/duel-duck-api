package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"go.uber.org/zap"
)

func (s *DuelService) CreateAndJoinDuel(
	ctx context.Context,
	duel *model.Duel,
	user *model.User,
	duelOwnerAnswer uint8,
) error {
	if user.Balance.LessThan(duel.DuelPrice) {
		return apperrors.BadRequest("not enough balance")
	}

	err := s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		err := s.DuelRepository.WithTx(tx).Create(ctx, duel)
		if err != nil {
			return apperrors.Internal("failed to create duel", err)
		}

		join := &model.JoinDuelReq{DuelID: duel.ID, Answer: duelOwnerAnswer}
		if _, err = s.DuelRepository.WithTx(tx).JoinDuel(ctx, user.ID, join, duel); err != nil {
			return apperrors.Internal("failed to join duel owner to the duel", err)
		}

		_, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, user.ID, -int(duel.DuelPrice))
		if err != nil {
			return apperrors.Internal("failed to alter user's balance", err)
		}

		if err = s.TournamentRepository.WithTx(tx).IncrementPlayersCount(ctx, user.ID, duel.TournamentID); err != nil {
			return apperrors.Internal("failed to increment players count in tournament", err)
		}

		if err = s.TournamentRepository.WithTx(tx).AlterLeaderboardSpent(ctx, user, duel); err != nil {
			return apperrors.Internal("failed to update tournament leaderboard", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Task Completion
	if duel.Topic == model.DuelTopicCrypto {
		if err = s.completeCreateDuelTask(ctx, duel.ID); err != nil {
			zap.L().Warn("failed to complete task of duel creation", zap.Any("duel_id", duel.ID), zap.Error(err))
		}
	}

	// Task Completion
	if err = s.completeReferAFriendTask(ctx, user.ID); err != nil {
		zap.L().Warn("failed to complete task of friend referring",
			zap.Any("user_id", user.ID), zap.Any("duel_id", duel.ID), zap.Error(err))
	}

	// Task Completion
	if err = s.completeDailyStreakTask(ctx, user.ID); err != nil {
		zap.L().Warn("failed to complete task of daily streak",
			zap.Any("user_id", user.ID), zap.Any("duel_id", duel.ID), zap.Error(err))
	}

	if err := s.sendDuelShareImageReq(duel); err != nil {
		zap.L().Error("failed on duel share image request", zap.Error(err))
	}

	return nil
}

func (s *DuelService) sendDuelShareImageReq(duel *model.Duel) error {
	resp, err := s.DuelShareClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(duel).
		Post(s.DuelShareImageAPI)
	if err != nil {
		return err
	}

	if resp.IsError() {
		return fmt.Errorf("status: %d, body: %s", resp.StatusCode(), resp.String())
	}

	return nil
}

func validatePaymentType(req *model.CreateDuelReq) error {
	if !req.PaymentType.IsValid() {
		return apperrors.BadRequest("invalid payment type")
	}

	var minPrice, maxPrice uint64

	switch req.PaymentType {
	case mtype.PaymentTypeDuck:
		minPrice = model.DuckPointsDuelMinJoinPrice
		maxPrice = model.DuckPointsDuelMaxJoinPrice
	case mtype.PaymentTypeUSDC:
		minPrice = model.USDCDuelMinJoinPrice
		maxPrice = model.USDCDuelMaxJoinPrice
	default:
		return apperrors.BadRequest("invalid payment type")
	}

	if req.DuelPrice < minPrice || req.DuelPrice > maxPrice {
		return apperrors.BadRequest("invalid duel price")
	}

	return nil
}

func validateDuelCancel(duel *model.Duel, req *model.DuelCancelReq) error {
	if req.Status != model.DuelStatusAdminCancelled && req.Status != model.DuelStatusRefund {
		return apperrors.BadRequest("cancel is not possible with provided status")
	}

	if req.Status == model.DuelStatusAdminCancelled && duel.Status != model.DuelStatusInReview {
		return apperrors.BadRequest("cancel is not possible from current duel status")
	}

	if req.Status == model.DuelStatusRefund && duel.Status != model.DuelStatusInProcess {
		return apperrors.BadRequest("refund is not possible from current duel status")
	}

	return nil
}

func (s *DuelService) CompleteJoinTournamentTask(ctx context.Context, userID uuid.UUID) error {
	joinedForFirstTime, err := s.DuelRepository.JoinedTournamentDuelFirstTime(ctx, userID)
	if err != nil {
		return apperrors.Internal("failed to check if user has created any duel before", err)
	}

	if !joinedForFirstTime {
		return nil
	}

	return err
}

func (s *DuelService) completeJoinDuelTask(ctx context.Context, userID uuid.UUID) error {
	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			return s.TaskService.Complete(
				ctx,
				tx,
				model.NewCompletedTask(
					userID,
					model.TaskJoinDuel,
				),
			)
		},
	)
	return err
}

func (s *DuelService) completeWinDuelTask(ctx context.Context, winnerIDs []uuid.UUID) error {
	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			for _, id := range winnerIDs {
				err := s.TaskService.Complete(
					ctx,
					tx,
					model.NewCompletedTask(
						id,
						model.TaskWinDuel,
					),
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)
	return err
}

func (s *DuelService) completeCreateDuelTask(ctx context.Context, duelID uuid.UUID) error {
	err := s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		userID, err := s.DuelRepository.FindOwnerIDByDuelID(ctx, duelID)
		if err != nil {
			return apperrors.Internal("failed to get owner id by duel id", err)
		}

		if userID == uuid.Nil {
			return apperrors.NotFound("duel id not found")
		}

		err = s.TaskService.Complete(
			ctx,
			tx,
			model.NewCompletedTask(
				userID,
				model.TaskCreateDuel,
			),
		)
		if err != nil {
			return err
		}

		return nil
	})
	return err
}

func (s *DuelService) completeDailyStreakTask(ctx context.Context, userID uuid.UUID) error {
	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			user, err := s.UserRepository.WithTx(tx).GetByID(ctx, userID)
			if err != nil {
				return apperrors.Internal("failed to get user by id", err)
			}

			reward := model.CurrentDailyReward(user)
			if reward.Claimed {
				// User has already completed task of daily streak today
				return nil
			}

			user.LastCompletedStreak = time.Now().UTC()
			user.DailyRewardStreak = reward.DayStreak

			rewardMultiplier := reward.RewardDayByStreak()

			if err = s.UserRepository.WithTx(tx).Update(ctx, user); err != nil {
				return apperrors.Internal("failed to update user", err)
			}

			return s.TaskService.Complete(
				ctx,
				tx,
				&model.CompletedTask{
					UserID:           userID,
					TaskID:           model.TaskCompletDailyStreak,
					RewardMultiplier: float64(rewardMultiplier),
				},
			)
		},
	)

	return err
}

func (s *DuelService) completeReferAFriendTask(ctx context.Context, userID uuid.UUID) error {
	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			// check has the referrer already completed task
			hasJoined, err := s.PlayerRepository.WithTx(tx).HasUserJoinedMoreThanOneDuel(
				ctx,
				userID,
			)
			if err != nil {
				return apperrors.Internal("failed to check if the user has joined more than one duel", err)
			}

			// User has already joined or created more than one duel
			// So his referrer already compeleted task
			if hasJoined {
				return nil
			}

			// Fetch referrer user id
			referrerID, err := s.ReferralRepository.WithTx(tx).GetReferrerID(ctx, userID)
			if err != nil {
				return apperrors.Internal("failed to get referrer id by referral id", err)
			}
			if referrerID == uuid.Nil {
				return nil
			}

			return s.TaskService.Complete(
				ctx,
				tx,
				model.NewCompletedTask(
					referrerID,
					model.TaskReferAFriend,
				),
			)
		},
	)

	return err
}
