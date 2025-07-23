package service

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/zap"

	"gitlab.com/duel-duck/duel-duck-api/config"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
)

type DuelService struct {
	TaskService              *TaskService
	WalletService            *WalletService
	CoinService              *CoinService
	TaskRepository           *repository.TaskRepository
	TournamentRepository     *repository.TournamentRepository
	UserRepository           *repository.UserRepository
	TxRepository             *repository.TransactionRepository
	DuelRepository           *repository.DuelRepository
	DuelEntityRepository     *repository.DuelEntityRepository
	ModeratorStatsRepository *repository.ModeratorStatsRepository
	PlayerRepository         *repository.PlayerRepository
	ReferralRepository       *repository.ReferralRepository
	DuelShareClient          *resty.Client
	TransactionManager       *repo.TransactionManager
	ContractAddress          string
	DuelShareImageAPI        string
}

func NewDuelService(
	c *config.Config,
	taskService *TaskService,
	walletService *WalletService,
	coinService *CoinService,
	taskRepository *repository.TaskRepository,
	tournamentRepository *repository.TournamentRepository,
	userRepository *repository.UserRepository,
	txRepository *repository.TransactionRepository,
	duelRepository *repository.DuelRepository,
	duelEntityRepository *repository.DuelEntityRepository,
	moderatorStatsRepository *repository.ModeratorStatsRepository,
	playerRepository *repository.PlayerRepository,
	referralRepository *repository.ReferralRepository,
	transactionManager *repo.TransactionManager,
) (*DuelService, error) {
	return &DuelService{
		TaskService:              taskService,
		WalletService:            walletService,
		CoinService:              coinService,
		TaskRepository:           taskRepository,
		TournamentRepository:     tournamentRepository,
		UserRepository:           userRepository,
		TxRepository:             txRepository,
		DuelRepository:           duelRepository,
		DuelEntityRepository:     duelEntityRepository,
		ModeratorStatsRepository: moderatorStatsRepository,
		PlayerRepository:         playerRepository,
		ReferralRepository:       referralRepository,
		DuelShareClient:          resty.New(),
		TransactionManager:       transactionManager,
		ContractAddress:          c.App.ContractAddress,
		DuelShareImageAPI:        c.App.DuelShareImageAPI,
	}, nil
}

func (s *DuelService) CreateNewDuel(
	ctx context.Context,
	userID uuid.UUID,
	req *model.CreateDuelReq,
) (*model.Duel, error) {
	if err := validatePaymentType(req); err != nil {
		return nil, err
	}

	if req.PaymentType != mtype.PaymentTypeDuck {
		return nil, apperrors.BadRequest("this endpoint is only used to create duck points duels")
	}

	user, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to find user", err)
	}

	duel := model.DuelByCreateReq(req, user)

	err = s.CreateAndJoinDuel(ctx, duel, user, req.Answer)
	if err != nil {
		return nil, err
	}

	return duel, nil
}

func (s *DuelService) CreateNewDuelAdmin(ctx context.Context, adminID uuid.UUID, req *model.CreateDuelReq) error {
	if err := validatePaymentType(req); err != nil {
		return err
	}

	if req.PaymentType != mtype.PaymentTypeDuck {
		return apperrors.BadRequest("this endpoint is only used to create duck points duels")
	}

	user, err := s.UserRepository.GetByID(ctx, adminID)
	if err != nil {
		return apperrors.Internal("failed to find user", err)
	}

	duel := model.DuelByCreateReqAdmin(req, user)

	if user.Balance.LessThan(duel.DuelPrice) {
		return apperrors.BadRequest("not enough balance")
	}

	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			err = s.DuelRepository.WithTx(tx).Create(ctx, duel)
			if err != nil {
				return err
			}

			join := &model.JoinDuelReq{DuelID: duel.ID, Answer: req.Answer}
			_, err = s.DuelRepository.WithTx(tx).JoinDuel(ctx, user.ID, join, duel)
			if err != nil {
				return err
			}

			_, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, user.ID, -int(duel.DuelPrice))
			if err != nil {
				return err
			}

			err = s.TournamentRepository.WithTx(tx).IncrementPlayersCount(ctx, user.ID, duel.TournamentID)
			if err != nil {
				return apperrors.Internal("failed to update tournament stats", err)
			}

			if err = s.TournamentRepository.WithTx(tx).AlterLeaderboardSpent(ctx, user, duel); err != nil {
				return apperrors.Internal("failed to update tournament leaderboard", err)
			}

			stats := &model.ModeratorStatsCreate{
				ModeratorID: adminID,
				DuelID:      duel.ID,
				ActionType:  model.ActionCreation,
				CreationPay: model.CreationPay,
			}
			return s.ModeratorStatsRepository.WithTx(tx).Create(ctx, stats)
		})
	if err != nil {
		return apperrors.Internal("failed to create duel", err)
	}

	if err := s.sendDuelShareImageReq(duel); err != nil {
		zap.L().Error("failed on duel share image request", zap.Error(err))
	}

	return nil
}

func (s *DuelService) JoinDuel(ctx context.Context, userID uuid.UUID, req *model.JoinDuelReq) (*model.Player, error) {
	user, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user", err)
	}

	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel", err)
	}

	if err = s.isAbleToJoinDuel(ctx, duel, userID); err != nil {
		return nil, err
	}

	if duel.PaymentType == mtype.PaymentTypeDuck {
		if user.Balance.LessThan(duel.DuelPrice) {
			return nil, apperrors.BadRequest("not enough balance")
		}
	} else {
		return nil, apperrors.BadRequest("this endpoint must be used to join only duck points duels")
	}

	var player *model.Player
	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			player, err = s.DuelRepository.WithTx(tx).JoinDuel(ctx, userID, req, duel)
			if err != nil {
				return apperrors.Internal("failed to join duel", err)
			}

			_, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, userID, -int(duel.DuelPrice))
			if err != nil {
				return apperrors.Internal("failed to alter user's balance", err)
			}

			err = s.TournamentRepository.WithTx(tx).IncrementPlayersCount(ctx, user.ID, duel.TournamentID)
			if err != nil {
				return apperrors.Internal("failed to update tournament stats", err)
			}

			if err = s.TournamentRepository.WithTx(tx).AlterLeaderboardSpent(ctx, user, duel); err != nil {
				return apperrors.Internal("failed to update tournament leaderboard", err)
			}

			return nil
		})

	if err != nil {
		return nil, apperrors.Internal("failed to join duel", err)
	}

	// Task Completion
	if err = s.completeJoinDuelTask(ctx, userID); err != nil {
		zap.L().Warn("failed to complete a task of duel joining", zap.Any("duel_id", duel.ID), zap.Error(err))
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

	return player, nil
}

func (s *DuelService) isAbleToJoinDuel(ctx context.Context, duel *model.Duel, userID uuid.UUID) error {
	if duel.Deadline.Before(time.Now()) {
		return apperrors.BadRequest("join is not possible since duel's deadline has passed")
	}

	duelIsInProgress := duel.Status == model.DuelStatusInProcess
	duelIsInReviewAndUserIsOwner := duel.Status == model.DuelStatusInReview && duel.OwnerID == userID

	if !(duelIsInProgress || duelIsInReviewAndUserIsOwner) {
		return apperrors.BadRequest("failed to join duel")
	}

	isPlayer, err := s.PlayerRepository.UserAlreadyParticipant(ctx, userID, duel.ID)
	if err != nil {
		return apperrors.Internal("failed to check if user is already participating in duel", err)
	}

	if isPlayer {
		return apperrors.BadRequest("user is already participating in this duel")
	}

	return nil
}

func (s *DuelService) GetDuelByIDUnauthorized(ctx context.Context, duelID uuid.UUID) (*model.DuelShow, error) {
	duel, err := s.DuelRepository.GetDuelShowByID(ctx, uuid.Nil, duelID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel", err)
	}

	return duel, nil
}

func (s *DuelService) GetDuelByID(ctx context.Context, duelID uuid.UUID, userID uuid.UUID, role mtype.Role) (*model.DuelShow, []model.PlayerShow, error) {
	duel, err := s.DuelRepository.GetDuelShowByID(ctx, userID, duelID)
	if err != nil {
		return nil, nil, apperrors.Internal("failed to get duel", err)
	}

	if !role.Admin() {
		if duel.Status < model.DuelStatusAutoCancelled && duel.OwnerID != userID {
			return nil, nil, apperrors.BadRequest("not enough rights", err)
		}
	}

	players, err := s.PlayerRepository.GetAllPlayersByDuelID(ctx, duelID, nil)
	if err != nil {
		return nil, nil, apperrors.Internal("failed to get players", err)
	}

	return duel, players, nil
}

func (s *DuelService) GetAllDuelsUnauthorized(ctx context.Context, options *repo.Options) ([]model.DuelShow, error) {
	duels, err := s.DuelRepository.GetAllDuels(ctx, uuid.Nil, options)
	if err != nil {
		return nil, apperrors.Internal("failed to get duels", err)
	}

	return duels, nil
}

func (s *DuelService) GetAllDuels(ctx context.Context, userID uuid.UUID, options *repo.Options) ([]model.DuelShow, error) {
	duels, err := s.DuelRepository.GetAllDuels(ctx, userID, options)
	if err != nil {
		return nil, apperrors.Internal("failed to get duels", err)
	}

	return duels, nil
}

func (s *DuelService) CountAllDuels(ctx context.Context, options *repo.Options) (int, error) {
	count, err := s.DuelRepository.CountWithOptions(ctx, options)
	if err != nil {
		return 0, apperrors.Internal("failed to count all duels", err)
	}

	return count, nil
}

func (s *DuelService) GetAllDuelsWhereParticipant(ctx context.Context, userID uuid.UUID, options *repo.Options) ([]model.DuelShow, error) {
	duels, err := s.DuelRepository.GetAllDuelsWhereParticipate(ctx, userID, options)
	if err != nil {
		return nil, apperrors.Internal("failed to get duels", err)
	}

	return duels, nil
}

func (s *DuelService) GetMyHistoryDuels(ctx context.Context, userID uuid.UUID, options *repo.Options) ([]model.DuelShow, error) {
	duels, err := s.DuelRepository.GetHistoryByUserID(ctx, userID, options)
	if err != nil {
		return nil, apperrors.Internal("failed to get duels history", err)
	}

	return duels, nil
}

func (s *DuelService) GetMyDuels(ctx context.Context, userID uuid.UUID, options *repo.Options) ([]model.DuelShow, error) {
	duels, err := s.DuelRepository.GetUserDuels(ctx, userID, options)
	if err != nil {
		return nil, apperrors.Internal("failed to get my duels", err)
	}

	return duels, nil
}

func (s *DuelService) GetDuelsCounterByTopic(
	ctx context.Context,
	opts *repo.Options,
) ([]model.DuelTopicCounter, error) {
	result, err := s.DuelRepository.GetTopicCounters(ctx, opts)
	if err != nil {
		return nil, apperrors.Internal("failed to get counters", err)
	}

	return result, nil
}

func (s *DuelService) GetIndividualDuelsCounterByTopic(
	ctx context.Context,
	opts repo.Options,
	userID uuid.UUID,
) ([]model.DuelTopicCounter, error) {
	result, err := s.DuelRepository.GetIndividualTopicCounters(ctx, opts, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to get individual duel counters counters", err)
	}

	return result, nil
}

func (s *DuelService) ResolveDuckPointDuelByAdmin(ctx context.Context,
	moderatorID uuid.UUID,
	req *model.DuelResolveReq,
) error {
	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return apperrors.Internal("failed to get duel by id", err)
	}

	if duel.Status != model.DuelStatusInProcess {
		return apperrors.BadRequest("resolve is not possible from current status")
	}

	if duel.PaymentType != mtype.PaymentTypeDuck {
		return apperrors.BadRequest("this endpoint should only be used to resolve duck point duels")
	}

	stats, err := model.NewModeratorStatsCreate(
		moderatorID,
		duel.ID,
		duel.Status,
		model.DuelStatusResolved)
	if err != nil {
		return err
	}
	duel.ResolvedBy = moderatorID

	params := &model.DuelResolveParams{
		DuelID:        req.DuelID,
		Answer:        req.Answer,
		JoinNotBefore: duel.Deadline,
	}

	if err = s.resolveDuckPointDuel(ctx, duel, params); err != nil {
		return err
	}

	if err = s.ModeratorStatsRepository.Create(ctx, stats); err != nil {
		return apperrors.Internal("failed to save moderator stats", err)
	}

	return nil
}

func (s *DuelService) resolveDuckPointDuel(
	ctx context.Context,
	duel *model.Duel,
	req *model.DuelResolveParams,
) error {
	duelWinners, err := s.PlayerRepository.GetDuelWinners(ctx, duel.ID, req.Answer, req.JoinNotBefore)
	if err != nil {
		return apperrors.Internal("failed to get players", err)
	}
	allDuelWinnersCount := uint64(len(duelWinners))

	playersToRefund, err := s.PlayerRepository.CountDuelPlayersToRefund(ctx, duel.ID, req.JoinNotBefore)
	if err != nil {
		return apperrors.Internal("count players that must be refunded", err)
	}

	playersPool := duel.PlayersCount - uint64(playersToRefund)
	if playersPool == 0 || playersPool == allDuelWinnersCount || allDuelWinnersCount == 0 {
		return s.cancelDuel(ctx, duel, model.AutoCancelReq(duel))
	}

	duelParams := model.NewDuelParams(
		duel.DuelPrice, duel.Commission,
		playersPool, allDuelWinnersCount,
	)

	commissionReward := duelParams.CalculateCommissionReward()
	winAmount := duelParams.CalculateFinalReward()

	duel.Status = model.DuelStatusResolved
	duel.FinalResult = &req.Answer
	duel.RefundedPlayersCount = uint64(playersToRefund)
	duel.WinnersCount = allDuelWinnersCount

	winnerIDs := make([]uuid.UUID, 0, len(duelWinners))
	for i := range duelWinners {
		winnerIDs = append(winnerIDs, duelWinners[i].UserID)
	}

	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			err = s.PlayerRepository.WithTx(tx).
				UpdateDuelWinners(ctx, duelWinners, winAmount)
			if err != nil {
				return err
			}

			err = s.UserRepository.WithTx(tx).
				UpdateDuelWinnersBalance(ctx, duel.ID, req.Answer, uint64(winAmount), req.JoinNotBefore)
			if err != nil {
				return err
			}

			err = s.TournamentRepository.WithTx(tx).AlterLeaderboardEarned(ctx, duel, winAmount, winnerIDs)
			if err != nil {
				return err
			}

			if commissionReward > 0 {
				_, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, duel.OwnerID, commissionReward)
				if err != nil {
					return err
				}

				referrerIncome := uint64(commissionReward / 2)
				err = s.ReferralRepository.WithTx(tx).UpdateReferrerDPIncomeByReferralID(ctx, duel.OwnerID, referrerIncome)
				if err != nil {
					return err
				}
			}

			if playersToRefund > 0 {
				err = s.UserRepository.WithTx(tx).
					PartialDuelRefund(ctx, duel.ID, duel.DuelPrice, req.JoinNotBefore)
				if err != nil {
					return err
				}

				err = s.PlayerRepository.WithTx(tx).MarkPlayersAsRefunded(ctx, duel.ID, req.JoinNotBefore)
				if err != nil {
					return err
				}
			}

			return s.DuelRepository.WithTx(tx).Update(ctx, duel)
		})

	if err != nil {
		return apperrors.Internal("failed to resolve a duel", err)
	}

	// Task Completion
	if err = s.completeWinDuelTask(ctx, winnerIDs); err != nil {
		zap.L().Warn("failed to complete a task of duel winning", zap.Any("duel_id", duel.ID), zap.Error(err))
	}

	return nil
}

func (s *DuelService) AutoCancelDuels(
	ctx context.Context,
) (uuid.UUIDs, error) {
	duels, err := s.DuelRepository.GetOldDuelsInReview(ctx, mtype.PaymentTypeDuck)
	if err != nil {
		return nil, apperrors.Internal("get old duck point duels in status review", err)
	}

	ids := make([]uuid.UUID, 0, len(duels))
	for i := range duels {
		ids = append(ids, duels[i].ID)

		req := &model.DuelCancelReq{DuelID: duels[i].ID, Status: model.DuelStatusAutoCancelled}
		if err = s.cancelDuel(ctx, &duels[i], req); err != nil {
			zap.L().Error("auto-cancel duck point duel",
				zap.String("id", duels[i].ID.String()),
				zap.Error(err))
		}
	}

	duels, err = s.DuelRepository.GetOldDuelsInReview(ctx, mtype.PaymentTypeUSDC)
	if err != nil {
		return nil, apperrors.Internal("get old crypto duels in status review", err)
	}

	for i := range duels {
		ids = append(ids, duels[i].ID)

		req := &model.DuelCancelReq{DuelID: duels[i].ID, Status: model.DuelStatusAutoCancelled}
		hashes, err := s.cancelCryptoDuel(ctx, &duels[i], req)
		if err != nil {
			zap.L().Error("auto-cancel crypto duel",
				zap.String("id", duels[i].ID.String()),
				zap.Strings("hashes", hashes),
				zap.Error(err))
		}
	}

	return ids, nil
}

func (s *DuelService) ApproveDuel(ctx context.Context,
	approvedBy uuid.UUID,
	req model.DuelApproveReq,
) error {
	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return apperrors.Internal("failed to get duel by id", err)
	}

	if duel.Status != model.DuelStatusInReview {
		return apperrors.BadRequest("resolve is not possible from current status")
	}

	duel.Status = model.DuelStatusInProcess
	duel.ApprovedBy = approvedBy
	err = s.DuelRepository.Update(ctx, duel)
	if err != nil {
		return apperrors.Internal("failed to resolve a duel", err)
	}

	// Task Completion
	if err = s.completeCreateDuelTask(ctx, duel.ID); err != nil {
		zap.L().Warn("failed to complete task of duel creation", zap.Any("duel_id", duel.ID), zap.Error(err))
	}

	return nil
}

func (s *DuelService) CancelDuckPointDuelByAdmin(ctx context.Context,
	moderatorID uuid.UUID,
	req *model.DuelCancelReq,
) error {
	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return apperrors.Internal("failed to get duel by id", err)
	}

	if duel.PaymentType != mtype.PaymentTypeDuck {
		return apperrors.BadRequest("this endpoint should only be used to cancel duck point duels")
	}

	if err = validateDuelCancel(duel, req); err != nil {
		return err
	}

	stats, err := model.NewModeratorStatsCreate(
		moderatorID,
		duel.ID,
		duel.Status,
		req.Status)
	if err != nil {
		return err
	}

	if err = s.cancelDuel(ctx, duel, req); err != nil {
		return err
	}

	if err = s.ModeratorStatsRepository.Create(ctx, stats); err != nil {
		return apperrors.Internal("create moderator stats", err)
	}

	return nil
}

func (s *DuelService) cancelDuel(
	ctx context.Context,
	duel *model.Duel,
	req *model.DuelCancelReq,
) error {
	duel.Status = req.Status
	duel.CancellationReason = req.CancellationReason

	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			err := s.UserRepository.WithTx(tx).RefundOnDuelCancel(ctx, duel.ID, duel.DuelPrice)
			if err != nil {
				return err
			}

			err = s.PlayerRepository.SetStatusToAll(ctx, duel.ID, model.PlayerStatusRefunded)
			if err != nil {
				return err
			}

			return s.DuelRepository.WithTx(tx).Update(ctx, duel)
		})
	if err != nil {
		return apperrors.Internal("failed to cancel a duel", err)
	}

	return nil
}

func (s *DuelService) EditDuel(ctx context.Context,
	req *model.DuelAdminEditReq,
) error {
	duel := &model.Duel{
		ID:            req.ID,
		Subtopic:      req.SubTopic,
		Question:      req.Question,
		Entities:      req.Entities,
		DuelType:      req.DuelType,
		SourceOfTruth: req.SourceOfTruth,
		Deadline:      req.Deadline,
		EventDate:     req.EventDate,
		ImageURL:      req.ImageURL,
		BgURL:         req.BgURL,
		DuelInfo:      req.DuelInfo,
	}

	err := s.DuelRepository.Update(ctx, duel)
	if err != nil {
		return apperrors.Internal("failed to edit duel", err)
	}

	return nil
}

func (s *DuelService) SetTournamentID(
	ctx context.Context,
	duelID uuid.UUID,
	tournamentID *uuid.UUID,
) error {
	err := s.DuelRepository.SetTournamentID(ctx, duelID, tournamentID)
	if err != nil {
		return apperrors.Internal("failed to edit duel", err)
	}

	return nil
}

func (s *DuelService) FindEntities(ctx context.Context,
	req *repo.Options,
) ([]model.DuelEntity, error) {
	entities, err := s.DuelEntityRepository.FindAll(ctx, req)
	if err != nil {
		return nil, apperrors.Internal("failed to find all duel entities", err)
	}

	return entities, nil
}

func (s *DuelService) CreateDuelEntity(
	ctx context.Context,
	req model.CreateDuelEntityReq,
) (*model.DuelEntity, error) {
	entity := &model.DuelEntity{
		SubtopicID:   req.SubtopicID,
		Name:         req.Name,
		EntityTypeID: req.EntityTypeID,
		ImageURL:     req.ImageURL,
	}

	entity, err := s.DuelEntityRepository.Create(ctx, entity)
	if repo.DuplicateKeyViolation(err) {
		return nil, apperrors.AlreadyExist("duel entity already exists", err)
	}

	if err != nil {
		return nil, apperrors.Internal("failed to create duel entity", err)
	}

	return entity, nil
}

func (s *DuelService) UpdateDuelEntity(
	ctx context.Context,
	newEntity *model.DuelEntity,
) error {
	err := s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		oldEntity, err := s.DuelEntityRepository.WithTx(tx).GetByID(ctx, newEntity.ID)
		if err != nil {
			return err
		}

		entityType, err := s.DuelEntityRepository.WithTx(tx).GetDuelEntityType(ctx, newEntity.EntityTypeID)
		if err != nil {
			return err
		}

		subtopic, err := s.DuelEntityRepository.WithTx(tx).GetDuelSubtopic(ctx, newEntity.SubtopicID)
		if err != nil {
			return err
		}

		err = s.DuelEntityRepository.WithTx(tx).Update(ctx, newEntity)
		if err != nil {
			return err
		}

		duelInfos, err := s.DuelRepository.WithTx(tx).GetAllByDuelInfoEntity(ctx, subtopic.Name, entityType)
		if err != nil {
			return err
		}

		if len(duelInfos) == 0 {
			return nil
		}

		// Double loop iz semi залуп
		for _, duel := range duelInfos {
			for i, entity := range duel.EntityValue {
				if entity == oldEntity.Name {
					duel.EntityValue[i] = newEntity.Name
					break
				}
			}
		}

		return s.DuelRepository.WithTx(tx).UpdateDuelInfoWithNewEntityName(ctx, entityType, duelInfos)
	})
	if repo.IsErrNoRows(err) {
		return apperrors.BadRequest("duel entity not found by id")
	}

	if repo.DuplicateKeyViolation(err) {
		return apperrors.AlreadyExist("duel entity already exists", err)
	}

	if err != nil {
		return apperrors.Internal("failed to update duel entity", err)
	}

	return nil
}

func (s *DuelService) FindSubtopics(ctx context.Context,
	req *repo.Options,
) ([]model.DuelSubtopic, error) {
	subtopics, err := s.DuelEntityRepository.FindAllDuelSubtopics(ctx, req)
	if err != nil {
		return nil, apperrors.Internal("failed to find all duel subtopics", err)
	}

	return subtopics, nil
}

func (s *DuelService) CreateDuelSubtopic(
	ctx context.Context,
	req model.CreateDuelSubtopicReq,
) (*model.DuelSubtopic, error) {
	subtopic := &model.DuelSubtopic{
		TopicID:  req.TopicID,
		Name:     req.Name,
		ImageURL: req.ImageURL,
	}

	subtopic, err := s.DuelEntityRepository.CreateDuelSubtopic(ctx, subtopic)
	if repo.DuplicateKeyViolation(err) {
		return nil, apperrors.AlreadyExist("duel subtopic already exists", err)
	}

	if err != nil {
		return nil, apperrors.Internal("failed to create duel subtopic", err)
	}

	return subtopic, nil
}

func (s *DuelService) UpdateDuelSubtopic(
	ctx context.Context,
	subtopic *model.DuelSubtopic,
) error {
	err := s.DuelEntityRepository.UpdateDuelSubtopic(ctx, subtopic)
	if repo.DuplicateKeyViolation(err) {
		return apperrors.AlreadyExist("duel subtopic already exists", err)
	}

	if err != nil {
		return apperrors.Internal("failed to update duel entity", err)
	}

	return nil
}

func (s *DuelService) ResolveDuckPointOnEventDate(ctx context.Context) (uuid.UUIDs, error) {
	duels, err := s.DuelRepository.GetEndedDuelsWithTopicCrypto(ctx, mtype.PaymentTypeDuck)
	if err != nil {
		return nil, apperrors.Internal("failed to get crypto duels with topic crypto", err)
	}

	if len(duels) == 0 {
		return nil, nil
	}

	resolveSelection := make([]uuid.UUID, 0, len(duels))
	for i := range duels {
		resolveSelection = append(resolveSelection, duels[i].ID)
	}
	zap.L().Info("selection of duck point duels for on_event_date resolve", zap.Any("ids", resolveSelection))

	resolvedDuels := make(uuid.UUIDs, 0, len(duels))
	for _, duel := range duels {
		err = s.resolveDuckPointOnEventDate(ctx, &duel)
		if err != nil {
			return nil, err
		}

		resolvedDuels = append(resolvedDuels, duel.ID)
	}

	return resolvedDuels, nil
}

func (s *DuelService) resolveDuckPointOnEventDate(ctx context.Context, duel *model.Duel) error {
	if duel.Topic != model.DuelTopicCrypto || duel.PaymentType != mtype.PaymentTypeDuck {
		return apperrors.BadRequest("failed to auto-resolve: duel has invalid topic or payment type")
	}

	info, ok := model.GetCryptoDuelInfo(duel.DuelInfo)
	if !ok {
		return apperrors.Internal("failed to resolve: duel has invalid duel info")
	}

	coinPrice, err := s.CoinService.GetCurrentCoinPriceByID(uint64(info.ID))
	if err != nil {
		return err
	}

	correctAnswer := info.DetermineWinningBet(coinPrice)
	req := &model.DuelResolveParams{
		DuelID:        duel.ID,
		Answer:        correctAnswer,
		JoinNotBefore: duel.Deadline,
	}

	return s.resolveDuckPointDuel(ctx, duel, req)
}

func (s *DuelService) ResolveDuckPointBeforeEventDate(
	ctx context.Context,
	joinNotBefore time.Duration,
) (uuid.UUIDs, error) {
	coinIDs, err := s.DuelRepository.GetAllCoinsFromActiveDuels(ctx)
	if err != nil {
		return nil, apperrors.Internal("failed to get all coins in active duels", err)
	}

	if len(coinIDs) == 0 {
		return nil, nil
	}

	highLow, err := s.CoinService.GetHighLowHourly(coinIDs...)
	if err != nil {
		return nil, err
	}

	duels, err := s.DuelRepository.GetActiveCryptoDuelsWithTopicCrypto(ctx, mtype.PaymentTypeDuck)
	if err != nil {
		return nil, apperrors.Internal("failed to get active crypto duels with topic crypto", err)
	}

	resolveSelection := make([]uuid.UUID, 0, len(duels))
	for i := range duels {
		resolveSelection = append(resolveSelection, duels[i].ID)
	}
	zap.L().Info("selection of duck point duels for before_event_date resolve", zap.Any("ids", resolveSelection))

	resolvedDuels := make(uuid.UUIDs, 0, len(duels))
	for _, duel := range duels {
		duelInfo, ok := model.GetCryptoDuelInfo(duel.DuelInfo)
		if !ok {
			zap.L().Error("failed to parse duelInfo for crypto duel",
				zap.String("id", duel.ID.String()),
			)
		}

		id := model.CMCID(duelInfo.ID)
		hl, ok := highLow[id]
		if !ok {
			continue
		}

		if duelInfo.ReachedPriceBeforeEventDate(hl.High, hl.Low) {
			req := &model.DuelResolveParams{
				DuelID:        duel.ID,
				Answer:        uint8(1),
				JoinNotBefore: time.Now().Add(-1 * joinNotBefore),
			}

			err = s.resolveDuckPointDuel(ctx, &duel, req)
			if err != nil {
				zap.L().Error("failed to resolve duck point duel before event_date",
					zap.Error(err),
					zap.String("duel_id", duel.ID.String()))
				return nil, err
			}

			resolvedDuels = append(resolvedDuels, duel.ID)
		}
	}

	return resolvedDuels, nil
}

func (s *DuelService) GetModeratorStats(
	ctx context.Context,
	opts *repo.Options,
	moderatorID uuid.UUID,
) ([]model.ModeratorStats, error) {
	stats, err := s.ModeratorStatsRepository.FindByModeratorIDWithOptions(ctx, opts, moderatorID)
	if err != nil {
		return nil, apperrors.Internal("failed to get moderator stats", err)
	}

	return stats, nil
}

func (s *DuelService) RefreshLeaderboard(ctx context.Context) error {
	if err := s.DuelRepository.RefreshMaterializedView(ctx, "leaderboard"); err != nil {
		return apperrors.Internal("failed to refresh leaderboard", err)
	}

	return nil
}

func (s *DuelService) RefreshLeaderboardTournament(
	ctx context.Context,
	tournamentName string,
) error {
	if err := s.DuelRepository.RefreshMaterializedView(ctx, tournamentName); err != nil {
		return apperrors.Internal("failed to refresh leaderboard", err)
	}

	return nil
}

func (s *DuelService) GetLeaderboard(ctx context.Context, opts repo.Options) ([]model.Leader, int, error) {
	leaders, leaderCount, err := s.DuelRepository.GetTotalLeaderboard(ctx, opts, "leaderboard")
	if err != nil {
		return nil, 0, apperrors.Internal("failed to get leaderboard", err)
	}

	return leaders, leaderCount, nil
}

func (s *DuelService) GetUserLeaderboardRank(
	ctx context.Context,
	opts repo.Options,
	userID uuid.UUID,
) (*model.Leader, error) {
	if !opts.Order.IsValid() {
		opts.Order = repo.Order{OrderBy: "pnl", OrderType: "desc"}
	}

	leader, err := s.DuelRepository.FindUserRank(ctx, opts, userID, "leaderboard")
	if err != nil {
		return nil, apperrors.Internal("failed to get user rank", err)
	}

	if leader == nil {
		return nil, apperrors.NotFound("user not found in leaderboard")
	}

	return leader, nil
}

type CountTournamentPlayersReq struct {
	Opts         repo.Options `json:"opts"`
	TournamentID uuid.UUID    `json:"tournament_id"`
}

func (s *DuelService) GetTransactionsBySignatures(
	ctx context.Context,
	signatures []string,
) ([]model.TransactionType, error) {
	transactions, err := s.TxRepository.GetTransactionsBySignatures(ctx, signatures)
	if err != nil {
		return nil, apperrors.Internal("failed to get transaction records", err)
	}

	return transactions, nil
}

// TODO: move GetTransactionsByPublicAddress to wallet service. And move transaction repository to wallet service from duel service

func (s *DuelService) GetTransactionsByPublicAddress(
	ctx context.Context,
	publicAddress string,
	req *model.GetTransactionHistoryReq,
) (*model.GetTransactionHistoryResp, error) {
	txs, err := s.WalletService.GetTransactions(ctx, publicAddress, req)
	if err != nil {
		return nil, err
	}

	signatures := make([]string, 0, len(txs))
	for _, tx := range txs {
		signatures = append(signatures, tx.TransID)
	}

	txTypes, err := s.TxRepository.GetTransactionsBySignatures(ctx, signatures)
	if err != nil {
		return nil, apperrors.Internal("failed to get transaction records", err)
	}

	return &model.GetTransactionHistoryResp{Transactions: txs, Types: txTypes}, nil
}

// TODO: move GetTokenAccounts to wallet service. And move transaction repository to wallet service from duel service

func (s *DuelService) GetTokenAccounts(
	ctx context.Context,
	publicAddress string,
	req *model.GetTokenAccountsReq,
) (*model.SolscanTokenAccountsResp, error) {
	txs, err := s.WalletService.GetTokenAccounts(ctx, publicAddress, req)
	if err != nil {
		return nil, err
	}

	return txs, nil
}
