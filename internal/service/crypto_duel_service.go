package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/storage/repository"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
	"go.uber.org/zap"
)

func (s *DuelService) CreateNewCryptoDuelAdmin(
	ctx context.Context,
	userID uuid.UUID,
	req *model.CreateDuelReq,
) (*model.Duel, error) {
	if err := validatePaymentType(req); err != nil {
		return nil, err
	}

	if req.PaymentType != mtype.PaymentTypeUSDC {
		return nil, apperrors.BadRequest("this endpoint is only used to create crypto duels")
	}

	user, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to find user", err)
	}

	duel := model.DuelByCreateReqAdmin(req, user)
	if err = s.DuelRepository.Create(ctx, duel); err != nil {
		return nil, apperrors.Internal("failed to create crypto duel", err)
	}

	if err := s.sendDuelShareImageReq(duel); err != nil {
		zap.L().Error("failed on duel share image request", zap.Error(err))
	}

	return duel, nil
}

func (s *DuelService) createAndJoinCryptoDuel(
	ctx context.Context,
	duel *model.Duel,
	user *model.User,
	duelOwnerAnswer uint8,
	txHash string,
) error {
	join := &model.JoinDuelReq{
		DuelID: duel.ID,
		Answer: duelOwnerAnswer,
	}

	err := s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			err := s.DuelRepository.WithTx(tx).Create(ctx, duel)
			if err != nil {
				return apperrors.Internal("failed to create duel", err)
			}

			_, err = s.DuelRepository.WithTx(tx).JoinDuel(ctx, user.ID, join, duel)
			if err != nil {
				return apperrors.Internal("failed to join duel owner to the duel", err)
			}

			err = s.TournamentRepository.WithTx(tx).IncrementPlayersCount(ctx, user.ID, duel.TournamentID)
			if err != nil {
				return apperrors.Internal("failed to update tournament stats", err)
			}

			if err = s.TournamentRepository.WithTx(tx).AlterLeaderboardSpent(ctx, user, duel); err != nil {
				return apperrors.Internal("failed to update tournament leaderboard", err)
			}

			if txHash != "" {
				txRecord := &model.TransactionType{Signature: txHash, TxType: model.TransactionTypeDuelPrediction}

				if err = s.TxRepository.WithTx(tx).Create(ctx, txRecord); err != nil {
					return apperrors.Internal("failed to join duel owner to the duel", err)
				}
			}

			return nil
		})
	if err != nil {
		return err
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

	return nil
}

func (s *DuelService) CreateNewCryptoDuel(
	ctx context.Context,
	userID uuid.UUID,
	req *model.CreateDuelReq,
) (*model.CreateCryptoDuelResp, error) {
	if err := validatePaymentType(req); err != nil {
		return nil, err
	}

	if req.PaymentType != mtype.PaymentTypeUSDC {
		return nil, apperrors.BadRequest("this endpoint is only used to create crypto duels")
	}

	user, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to find user", err)
	}

	duel := model.DuelByCreateReq(req, user)

	if duel.Status == model.DuelStatusInReview {
		if err = s.createAndJoinCryptoDuel(ctx, duel, user, req.Answer, ""); err != nil {
			return nil, err
		}

		return &model.CreateCryptoDuelResp{Duel: duel}, nil
	}

	if duel.Status != model.DuelStatusInProcess {
		return nil, apperrors.Internal("got invalid status on duel creation")
	}

	resp, err := s.WalletService.InitAndJoinSolanaRoom(ctx, duel, user, req.Answer)
	if err != nil {
		return nil, err
	}

	if err = s.createAndJoinCryptoDuel(ctx, duel, user, req.Answer, resp.TxHash); err != nil {
		return nil, err
	}

	if err := s.sendDuelShareImageReq(duel); err != nil {
		zap.L().Error("failed on duel share image request", zap.Error(err))
	}

	return &model.CreateCryptoDuelResp{
		Duel:   duel,
		Result: resp,
	}, nil
}

func (s *DuelService) JoinCryptoDuel(
	ctx context.Context,
	userID uuid.UUID,
	req *model.JoinDuelReq,
) (*model.JoinCryptoDuelResp, error) {
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
		return nil, apperrors.BadRequest("this endpoint must be used to join only crypto duels")
	}

	result := new(model.JoinSolanaRoomResp)
	if duel.PlayersCount == 0 {
		result, err = s.WalletService.InitAndJoinSolanaRoom(ctx, duel, user, req.Answer)
	} else {
		result, err = s.WalletService.JoinSolanaRoom(ctx, duel, user, req.Answer)
	}
	if err != nil {
		return nil, err
	}

	var player *model.Player
	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		player, err = s.DuelRepository.WithTx(tx).JoinDuel(ctx, userID, req, duel)
		if err != nil {
			return apperrors.Internal("failed to join duel", err)
		}

		err = s.TournamentRepository.WithTx(tx).IncrementPlayersCount(ctx, user.ID, duel.TournamentID)
		if err != nil {
			return apperrors.Internal("failed to update tournament stats", err)
		}

		if err = s.TournamentRepository.AlterLeaderboardSpent(ctx, user, duel); err != nil {
			return apperrors.Internal("failed to update tournament leaderboard", err)
		}

		txRecord := &model.TransactionType{Signature: result.TxHash, TxType: model.TransactionTypeDuelPrediction}
		if err = s.TxRepository.WithTx(tx).Create(ctx, txRecord); err != nil {
			return apperrors.Internal("failed to create transaction record", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if duel.TournamentID.String() == repository.TournamentID {
		if err = s.CompleteJoinTournamentTask(ctx, userID); err != nil {
			zap.L().Error("failed to complete tournament join task", zap.Error(err))
		}
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

	return &model.JoinCryptoDuelResp{Player: player, Result: result}, nil
}

func (s *DuelService) ResolveCryptoBeforeEventDate(
	ctx context.Context,
	joinNotBefore time.Duration,
) (map[uuid.UUID][]string, error) {
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

	duels, err := s.DuelRepository.GetActiveCryptoDuelsWithTopicCrypto(ctx, mtype.PaymentTypeUSDC)
	if err != nil {
		return nil, apperrors.Internal("failed to get active crypto duels with topic crypto", err)
	}

	resolveSelection := make([]uuid.UUID, 0, len(duels))
	for i := range duels {
		resolveSelection = append(resolveSelection, duels[i].ID)
	}
	zap.L().Info("selection of crypto duels for before_event_date resolve", zap.Any("ids", resolveSelection))

	duelResolveHashes := make(map[uuid.UUID][]string, len(duels))
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

			txHashes, err := s.resolveCryptoDuel(ctx, &duel, req)
			if err != nil {
				zap.L().Error("failed to resolve crypto duel before event_date",
					zap.Error(err),
					zap.String("duel_id", duel.ID.String()))
				return nil, err
			}

			duelResolveHashes[duel.ID] = txHashes
		}
	}

	return duelResolveHashes, nil
}

func (s *DuelService) ResolveCryptoOnEventDate(ctx context.Context) (map[uuid.UUID][]string, error) {
	duels, err := s.DuelRepository.GetEndedDuelsWithTopicCrypto(ctx, mtype.PaymentTypeUSDC)
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
	zap.L().Info("selection of crypto duels for on_event_date resolve", zap.Any("ids", resolveSelection))

	duelResolveHashes := make(map[uuid.UUID][]string, len(duels))
	for _, duel := range duels {
		txHashes, err := s.resolveCryptoOnDeadline(ctx, &duel)
		if err != nil {
			return nil, err
		}

		if _, ok := duelResolveHashes[duel.ID]; ok {
			return nil, apperrors.Internal("tried to resolve already resolved duel")
		}
		duelResolveHashes[duel.ID] = txHashes
	}

	return duelResolveHashes, nil
}

func (s *DuelService) resolveCryptoOnDeadline(ctx context.Context, duel *model.Duel) ([]string, error) {
	if duel.Topic != model.DuelTopicCrypto || duel.PaymentType == mtype.PaymentTypeDuck {
		return nil, apperrors.BadRequest("failed to auto-resolve: duel has invalid topic or payment type")
	}

	info, ok := model.GetCryptoDuelInfo(duel.DuelInfo)
	if !ok {
		return nil, apperrors.Internal("failed to resolve: duel has invalid duel info")
	}

	coinPrice, err := s.CoinService.GetCurrentCoinPriceByID(uint64(info.ID))
	if err != nil {
		return nil, err
	}

	correctAnswer := info.DetermineWinningBet(coinPrice)
	req := &model.DuelResolveParams{
		DuelID:        duel.ID,
		Answer:        correctAnswer,
		JoinNotBefore: duel.Deadline,
	}

	return s.resolveCryptoDuel(ctx, duel, req)
}

func (s *DuelService) ResolveCryptoDuelByAdmin(
	ctx context.Context,
	moderatorID uuid.UUID,
	req *model.DuelResolveReq,
) ([]string, error) {
	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel by id", err)
	}

	if duel.Status != model.DuelStatusInProcess {
		return nil, apperrors.BadRequest("resolve is not possible from current status")
	}

	if duel.PaymentType == mtype.PaymentTypeDuck {
		return nil, apperrors.BadRequest("this endpoint should only be used to resolve crypto duels")
	}

	params := &model.DuelResolveParams{
		DuelID:        duel.ID,
		Answer:        req.Answer,
		JoinNotBefore: duel.Deadline,
	}
	txHashes, err := s.resolveCryptoDuelByAdmin(ctx, moderatorID, duel, params)
	if err != nil {
		return nil, err
	}

	return txHashes, nil
}

func (s *DuelService) resolveCryptoDuelByAdmin(
	ctx context.Context,
	moderatorID uuid.UUID,
	duel *model.Duel,
	params *model.DuelResolveParams,
) ([]string, error) {
	stats, err := model.NewModeratorStatsCreate(moderatorID, duel.ID, duel.Status, model.DuelStatusResolved)
	if err != nil {
		return nil, err
	}
	duel.ResolvedBy = moderatorID

	txHashes, err := s.resolveCryptoDuel(ctx, duel, params)
	if err != nil {
		return nil, err
	}

	if err = s.ModeratorStatsRepository.Create(ctx, stats); err != nil {
		return nil, err
	}

	return txHashes, nil
}

func (s *DuelService) resolveCryptoDuel(
	ctx context.Context,
	duel *model.Duel,
	req *model.DuelResolveParams,
) ([]string, error) {
	duelWinners, err := s.PlayerRepository.GetDuelWinners(ctx, duel.ID, req.Answer, req.JoinNotBefore)
	if err != nil {
		return nil, apperrors.Internal("failed to count players with specific answer", err)
	}
	allDuelWinnersCount := uint64(len(duelWinners))

	playersToRefund, err := s.PlayerRepository.CountDuelPlayersToRefund(ctx, duel.ID, req.JoinNotBefore)
	if err != nil {
		return nil, apperrors.Internal("failed to count players that must be refunded", err)
	}

	playersPool := duel.PlayersCount - uint64(playersToRefund)
	if playersPool == 0 || playersPool == allDuelWinnersCount || allDuelWinnersCount == 0 {
		return s.cancelCryptoDuel(ctx, duel, model.AutoCancelReq(duel))
	}

	var refundedPlayersTxHashes []string
	if playersToRefund > 0 {
		refundedPlayersTxHashes, err = s.partialCryptoRefund(ctx, duel, req.JoinNotBefore)
		if err != nil {
			return nil, err
		}
	}

	unpaidWinners, err := s.PlayerRepository.GetCryptoDuelWinners(ctx, duel.ID, req.Answer)
	if err != nil {
		return nil, apperrors.Internal("failed to count players with specific answer", err)
	}

	if len(unpaidWinners) == 0 {
		return []string{}, nil
	}

	playersCount := duel.PlayersCount - duel.RefundedPlayersCount
	duelParams := model.NewDuelParams(
		duel.DuelPrice, duel.Commission,
		playersCount, allDuelWinnersCount,
	)

	winAmount := duelParams.CalculateFinalCryptoReward()
	duelRewardTxHashes, err := s.WalletService.RewardDuelWinners(ctx, winAmount, unpaidWinners)
	if err != nil {
		return nil, err
	}

	duelOwner, err := s.UserRepository.GetByID(ctx, duel.OwnerID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel owner by id", err)
	}

	creatorCommissionReward := duelParams.CalculateCryptoCommissionReward()

	referrer, err := s.ReferralRepository.ReferrerWalletByUserID(ctx, duelOwner.ID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel owner referrer by user id", err)
	}

	referrerPublicAddress, referrerCommissionReward := "", uint64(0)
	if referrer != nil {
		referrerPublicAddress = referrer.ReferrerPublicAddress
		referrerCommissionReward = creatorCommissionReward / 2
	}

	var creatorCommissionTxHash string
	if !duelOwner.Role.Admin() {
		creatorCommissionTxHash, err = s.WalletService.RewardDuelOwnerWithCommission(
			ctx,
			duelOwner.PublicAddress,
			creatorCommissionReward,
		)
		if err != nil {
			return nil, err
		}
	}

	referrerCommissionTxHash, err := s.WalletService.RewardDuelOwnerWithCommission(
		ctx,
		referrerPublicAddress,
		referrerCommissionReward,
	)
	if err != nil {
		return nil, err
	}

	txHash, err := s.WalletService.CloseSolanaRoom(ctx, duel.RoomNumber)
	if err != nil {
		zap.L().Error("failed to close solana room after resolve",
			zap.Error(err),
			zap.String("duel_id", duel.ID.String()),
			zap.Uint64("room_number", duel.RoomNumber))
	}
	txRecords := make([]model.TransactionType, 0, len(duelRewardTxHashes)+len(refundedPlayersTxHashes)+1)

	txRecords = append(
		txRecords,
		model.NewTransactionsWithSameType(
			model.TransactionTypeDuelReward,
			duelRewardTxHashes...,
		)...,
	)
	if len(refundedPlayersTxHashes) > 0 {
		txRecords = append(
			txRecords,
			model.NewTransactionsWithSameType(
				model.TransactionTypeDuelRefund,
				refundedPlayersTxHashes...,
			)...,
		)
	}

	if creatorCommissionTxHash != "" {
		txRecords = append(
			txRecords,
			model.NewTransactionsWithSameType(
				model.TransactionTypeDuelCommission,
				creatorCommissionTxHash,
				referrerCommissionTxHash,
			)...,
		)
	}

	duel.Status = model.DuelStatusResolved
	duel.FinalResult = &req.Answer
	duel.WinnersCount = allDuelWinnersCount

	playersWinAmount := float64(winAmount) / float64(model.USDCPriceMultiplier)

	winnerIDs := make([]uuid.UUID, 0, len(duelWinners))
	for i := range duelWinners {
		winnerIDs = append(winnerIDs, duelWinners[i].UserID)
	}

	err = s.TransactionManager.WithinTransaction(ctx,
		func(ctx context.Context, tx bun.Tx) error {
			err = s.PlayerRepository.WithTx(tx).UpdateDuelWinners(ctx, duelWinners, playersWinAmount)
			if err != nil {
				return err
			}

			err = s.TournamentRepository.WithTx(tx).AlterLeaderboardEarned(ctx, duel, playersWinAmount, winnerIDs)
			if err != nil {
				return err
			}

			if err = s.DuelRepository.WithTx(tx).Update(ctx, duel); err != nil {
				return err
			}

			if referrer != nil {
				rawCommission := float64(referrerCommissionReward) / float64(model.USDCPriceMultiplier)
				err = s.ReferralRepository.UpdateReferrerUSDCIncomeByReferralID(
					ctx,
					referrer.ReferrerID,
					rawCommission,
				)
				if err != nil {
					return err
				}
			}

			return s.TxRepository.WithTx(tx).BulkInsert(ctx, txRecords)
		})
	if err != nil {
		return nil, apperrors.Internal("failed to resolve a duel", err)
	}

	allTxHashes := append(duelRewardTxHashes, creatorCommissionTxHash, txHash)
	allTxHashes = append(allTxHashes, refundedPlayersTxHashes...)

	// Task Completion
	if err = s.completeWinDuelTask(ctx, winnerIDs); err != nil {
		zap.L().Warn("failed to complete a task of duel winning", zap.Any("duel_id", duel.ID), zap.Error(err))
	}

	return allTxHashes, nil
}

func (s *DuelService) ApproveCryptoDuel(ctx context.Context,
	approvedBy uuid.UUID,
	req model.DuelApproveReq,
) (*model.JoinCryptoDuelResp, error) {
	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel by id", err)
	}

	if duel.Status != model.DuelStatusInReview {
		return nil, apperrors.BadRequest("resolve is not possible from current status")
	}

	user, err := s.UserRepository.GetByID(ctx, duel.OwnerID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user by id", err)
	}

	player, err := s.PlayerRepository.GetByUserID(ctx, user.ID, duel.ID)
	if err != nil {
		return nil, apperrors.Internal("failed to get player by id", err)
	}

	result, err := s.WalletService.InitAndJoinSolanaRoom(ctx, duel, user, player.Answer)
	if err != nil {
		return nil, err
	}

	duel.Status = model.DuelStatusInProcess
	duel.ApprovedBy = approvedBy
	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		err = s.DuelRepository.Update(ctx, duel)
		if err != nil {
			return apperrors.Internal("failed to resolve a duel", err)
		}

		txRecord := &model.TransactionType{Signature: result.TxHash, TxType: model.TransactionTypeDuelPrediction}
		if err = s.TxRepository.WithTx(tx).Create(ctx, txRecord); err != nil {
			return apperrors.Internal("failed to create transaction record", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Task Completion
	if err = s.completeCreateDuelTask(ctx, duel.ID); err != nil {
		zap.L().Warn("failed to complete task of duel creation", zap.Any("duel_id", duel.ID), zap.Error(err))
	}

	return &model.JoinCryptoDuelResp{
		Player: player,
		Result: result,
	}, nil
}

func (s *DuelService) CancelCryptoDuelByAdmin(ctx context.Context,
	resolvedBy uuid.UUID,
	req *model.DuelCancelReq,
) ([]string, error) {
	duel, err := s.DuelRepository.GetByID(ctx, req.DuelID)
	if err != nil {
		return nil, apperrors.Internal("failed to get duel by id", err)
	}

	if duel.PaymentType == mtype.PaymentTypeDuck {
		return nil, apperrors.BadRequest("this endpoint should only be used to cancel crypto duels")
	}

	if err = validateDuelCancel(duel, req); err != nil {
		return nil, err
	}

	txHashes, err := s.cancelCryptoDuelByAdmin(ctx, resolvedBy, duel, req)
	if err != nil {
		return nil, err
	}

	err = s.TxRepository.BulkInsertWithSameTxType(ctx, model.TransactionTypeDuelRefund, txHashes)
	if err != nil {
		return nil, apperrors.Internal("failed to create transaction records", err)
	}

	return txHashes, nil
}

func (s *DuelService) cancelCryptoDuelByAdmin(ctx context.Context,
	resolvedBy uuid.UUID,
	duel *model.Duel,
	req *model.DuelCancelReq,
) ([]string, error) {
	stats, err := model.NewModeratorStatsCreate(resolvedBy, duel.ID, duel.Status, req.Status)
	if err != nil {
		return nil, err
	}

	txSignatures, err := s.cancelCryptoDuel(ctx, duel, req)
	if err != nil {
		return nil, err
	}

	if err = s.ModeratorStatsRepository.Create(ctx, stats); err != nil {
		return nil, apperrors.Internal("failed to update moderator stats", err)
	}

	return txSignatures, nil
}

func (s *DuelService) cancelCryptoDuel(ctx context.Context,
	duel *model.Duel,
	req *model.DuelCancelReq,
) ([]string, error) {
	var (
		txHashes          = make([]string, 0)
		roomClosingTxHash = ""
	)

	// If duel is in review, and it should be cancelled,
	// we don't have to return any money, since we didn't charge anything from user
	if HasChargedDuelPriceFromUser(duel.PlayersCount, duel.Status, req.Status) {
		players, err := s.PlayerRepository.GetCryptoDuelPlayers(ctx, duel.ID)
		if err != nil {
			return nil, apperrors.Internal("failed to get duel players", err)
		}

		txHashes, err = s.WalletService.TransferUSDCBulk(ctx, duel.DuelPrice*model.USDCPriceMultiplier, players)
		if err != nil {
			return nil, apperrors.ServiceUnavailable("failed to refund duel: "+duel.ID.String(), err)
		}

		roomClosingTxHash, err = s.WalletService.CloseSolanaRoom(ctx, duel.RoomNumber)
		if err != nil {
			zap.L().Error("failed to close solana room after refund",
				zap.Error(err),
				zap.String("duel_id", duel.ID.String()),
				zap.Uint64("room_number", duel.RoomNumber))
		}
	}

	duel.Status = req.Status
	duel.CancellationReason = req.CancellationReason

	err := s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		err := s.DuelRepository.Update(ctx, duel)
		if err != nil {
			return apperrors.Internal("failed to update duel status", err)
		}

		err = s.PlayerRepository.SetStatusToAll(ctx, duel.ID, model.PlayerStatusRefunded)
		if err != nil {
			return apperrors.Internal("failed to update crypto players status", err)
		}

		if len(txHashes) > 0 {
			err = s.TxRepository.WithTx(tx).BulkInsertWithSameTxType(
				ctx,
				model.TransactionTypeDuelRefund,
				txHashes)
			if err != nil {
				return apperrors.Internal("failed to create transaction records", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return append(txHashes, roomClosingTxHash), nil
}

func (s *DuelService) partialCryptoRefund(
	ctx context.Context,
	duel *model.Duel,
	votedAfter time.Time,
) ([]string, error) {
	if duel.Status != model.DuelStatusInProcess {
		return []string{}, nil
	}

	players, err := s.PlayerRepository.GetDuelPlayersToRefund(ctx, duel.ID, votedAfter)
	if err != nil {
		return nil, apperrors.Internal("failed to get crypto duel players", err)
	}

	txHashes, err := s.WalletService.TransferUSDCBulk(ctx, duel.DuelPrice*model.USDCPriceMultiplier, players)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to refund duel: "+duel.ID.String(), err)
	}

	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		err = s.PlayerRepository.WithTx(tx).SetStatus(ctx, players, model.PlayerStatusRefunded)
		if err != nil {
			return apperrors.Internal("failed to update crypto players status", err)
		}

		duel.RefundedPlayersCount += uint64(len(players))
		if err = s.DuelRepository.WithTx(tx).Update(ctx, duel); err != nil {
			return apperrors.Internal("failed to update duel", err)
		}

		err = s.TxRepository.WithTx(tx).BulkInsertWithSameTxType(
			ctx,
			model.TransactionTypeDuelRefund,
			txHashes)
		if err != nil {
			return apperrors.Internal("failed to create transaction records", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return txHashes, nil
}

func HasChargedDuelPriceFromUser(duelPlayersCount uint64, duelOldStatus, duelNewStatus uint8) bool {
	duelIsInProcess := duelOldStatus == model.DuelStatusInProcess
	isNewStatusRefund := duelNewStatus == model.DuelStatusRefund

	return duelIsInProcess && isNewStatusRefund && duelPlayersCount > 0
}
