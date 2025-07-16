package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-resty/resty/v2"
	"github.com/goccy/go-json"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	"time"
)

func (s *WalletService) InitSolanaRoom(ctx context.Context, duel *model.Duel) (string, error) {
	duelPrice := duel.DuelPrice * model.USDCPriceMultiplier
	reqBody := map[string]interface{}{
		"theme":       duel.Topic,
		"description": duel.Question,
		"percent":     uint32(duel.Commission),
		"bet":         uint32(duelPrice),
		"end":         uint32(duel.Deadline.Unix()),
		"pda_nr":      uint32(duel.RoomNumber),
	}

	instructions, err := s.GetInstructionsFromContractService(ctx, reqBody, "init", s.adminPrivateKey)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	sig, err := s.sendTransaction(ctx,
		tx,
		txSignerPrivateKeyGetter(s.adminPrivateKey))
	if err != nil {
		return "", apperrors.ServiceUnavailable("failed to send transaction", err)
	}

	return sig.String(), nil
}

func (s *WalletService) InitAndJoinSolanaRoom(
	ctx context.Context,
	duel *model.Duel,
	user *model.User,
	answer uint8,
) (string, error) {
	duelPrice := duel.DuelPrice * model.USDCPriceMultiplier

	reqBody := map[string]interface{}{
		"theme":       duel.Topic,
		"description": duel.Question,
		"percent":     uint32(duel.Commission),
		"bet":         uint32(duelPrice),
		"end":         uint32(duel.Deadline.Unix()),
		"pda_nr":      uint32(duel.RoomNumber),
	}

	txInit, err := s.GetTxFromContractService(reqBody, "init")
	if err != nil {
		return "", err
	}

	userPrivateKeyBase58, err := s.privateKeyRepository.GetPrivateKeyBase58(ctx, user.ID)
	if err != nil {
		return "", apperrors.Internal("failed to find users private key", err)
	}

	userPrivateKey, err := solana.PrivateKeyFromBase58(userPrivateKeyBase58)
	if err != nil {
		return "", apperrors.Internal("failed to parse user's private key", err)
	}

	userTokenAccount, _, err := solana.FindAssociatedTokenAddress(userPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil {
		return "", apperrors.Internal("failed to get user associated token address", err)
	}

	balance, err := s.GetTokenBalance(ctx, userTokenAccount)
	if err != nil {
		return "", apperrors.Internal("failed to get user associated token balance", err)
	}

	var autoswapNeeded bool
	if balance < duelPrice {
		autoswapNeeded = true
		requiredAmount := duelPrice - balance

		err := s.AutoswapUSDC(ctx, user.ID, userPrivateKey, requiredAmount)
		if err != nil {
			return "", err
		}
	}

	reqBody = map[string]interface{}{
		"multiplier": 1,
		"answer":     answer,
		"pda_nr":     duel.RoomNumber,
		"payer":      userPrivateKey.PublicKey().String(),
	}

	txJoin, err := s.GetTxFromContractService(reqBody, "join")
	if err != nil {
		return "", err
	}

	instructions, err := GetTxInstructions(txInit, txJoin)
	if err != nil {
		return "", err
	}

	tx, err := s.NewTransactionForSimulation(
		instructions,
		txTwoSignersPrivateKeyGetter(s.adminPrivateKey, userPrivateKey),
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()),
		solana.TransactionPayer(userPrivateKey.PublicKey()))
	if err != nil {
		return "", err
	}

	if autoswapNeeded {
		ok, err := s.waitForEnoughTokenBalance(ctx, userTokenAccount, duelPrice, 10*time.Second, 1*time.Second)
		if err != nil {
			return "", err
		}

		if !ok {
			return "", apperrors.BadRequest("not enough token balance (timeout)")
		}

		// tmp solution for mvp.
		// Sleep lets solana node have enough time to get user's updated balance after token swaps
		time.Sleep(60 * time.Second)
	}

	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil {
		return "", err
	}

	computeUnits = uint32(float64(computeUnits) * CUExtraCapacityCoefficient)
	cuPriceInstruction, err := computebudget.NewSetComputeUnitPriceInstructionBuilder().
		SetMicroLamports(s.PriorityTracker.GetMediumPriorityMicroLamports()).
		ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to set transaction compute unit price", err)
	}

	cuLimitInstruction, err := computebudget.NewSetComputeUnitLimitInstructionBuilder().
		SetUnits(computeUnits).
		ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to set transaction compute unit limit", err)
	}

	instructions = append([]solana.Instruction{cuPriceInstruction, cuLimitInstruction}, instructions...)

	tx, err = solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()),
		solana.TransactionPayer(userPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.ServiceUnavailable("failed to generate a transaction", err)
	}

	txHash, err := s.sendTxWithTracker(ctx,
		tx,
		txTwoSignersPrivateKeyGetter(s.adminPrivateKey, userPrivateKey))
	if err != nil {
		return "", apperrors.ServiceUnavailable("failed to send transaction", err)
	}

	return txHash.String(), nil
}

func (s *WalletService) JoinSolanaRoom(
	ctx context.Context,
	duel *model.Duel,
	user *model.User,
	answer uint8,
) (string, error) {
	multiplier := (duel.PlayersCount)/10 + 1
	if multiplier == (duel.PlayersCount-1)/10+1 {
		return s.joinSolanaRoom(ctx, duel, user, answer)
	}

	userPrivateKeyBase58, err := s.privateKeyRepository.GetPrivateKeyBase58(ctx, user.ID)
	if err != nil {
		return "", apperrors.Internal("failed to find users private key", err)
	}

	userPrivateKey, err := solana.PrivateKeyFromBase58(userPrivateKeyBase58)
	if err != nil {
		return "", apperrors.Internal("failed to parse user's private key", err)
	}

	userTokenAccount, _, err := solana.FindAssociatedTokenAddress(userPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil {
		return "", apperrors.Internal("failed to get user associated token address", err)
	}

	hasEnough, err := s.HasEnoughTokenBalance(
		ctx,
		userTokenAccount,
		duel.DuelPrice*model.USDCPriceMultiplier)
	if err != nil {
		return "", err
	}
	if !hasEnough {
		return "", apperrors.BadRequest("not enough balance to proceed a transaction")
	}

	reqBody := map[string]any{"pda_nr": duel.RoomNumber}
	txReallocate, err := s.GetTxFromContractService(reqBody, "reallocate")
	if err != nil {
		return "", err
	}

	reqBody = map[string]any{
		"multiplier": multiplier,
		"answer":     answer,
		"pda_nr":     duel.RoomNumber,
		"payer":      userPrivateKey.PublicKey().String(),
	}
	txJoin, err := s.GetTxFromContractService(reqBody, "join")
	if err != nil {
		return "", err
	}

	instructions, err := GetTxInstructions(txReallocate, txJoin)
	if err != nil {
		return "", err
	}

	tx, err := s.NewTransactionForSimulation(
		instructions,
		txTwoSignersPrivateKeyGetter(s.adminPrivateKey, userPrivateKey),
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()),
		solana.TransactionPayer(userPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil {
		return "", err
	}

	computeUnits = uint32(float64(computeUnits) * CUExtraCapacityCoefficient)
	cuPriceInstruction, err := computebudget.NewSetComputeUnitPriceInstructionBuilder().
		SetMicroLamports(s.PriorityTracker.GetMediumPriorityMicroLamports()).
		ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to set transaction compute unit price", err)
	}

	cuLimitInstruction, err := computebudget.NewSetComputeUnitLimitInstructionBuilder().
		SetUnits(computeUnits).
		ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to set transaction compute unit limit", err)
	}

	instructions = append([]solana.Instruction{cuPriceInstruction, cuLimitInstruction}, instructions...)

	tx, err = solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()),
		solana.TransactionPayer(userPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	txHash, err := s.sendTxWithTracker(ctx,
		tx,
		txTwoSignersPrivateKeyGetter(s.adminPrivateKey, userPrivateKey))
	if err != nil {
		return "", apperrors.ServiceUnavailable("failed to send transaction", err)
	}

	return txHash.String(), nil
}

func (s *WalletService) joinSolanaRoom(ctx context.Context, duel *model.Duel, user *model.User, answer uint8) (string, error) {
	userPrivateKeyBase58, err := s.privateKeyRepository.GetPrivateKeyBase58(ctx, user.ID)
	if err != nil {
		return "", apperrors.Internal("failed to find users private key", err)
	}

	userPrivateKey, err := solana.PrivateKeyFromBase58(userPrivateKeyBase58)
	if err != nil {
		return "", apperrors.Internal("failed to parse user's private key", err)
	}

	userTokenAccount, _, err := solana.FindAssociatedTokenAddress(userPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil {
		return "", apperrors.Internal("failed to get user associated token address", err)
	}

	hasEnough, err := s.HasEnoughTokenBalance(ctx, userTokenAccount, duel.DuelPrice)
	if err != nil {
		return "", err
	}
	if !hasEnough {
		return "", apperrors.BadRequest("not enough balance to proceed a transaction")
	}

	multiplier := (duel.PlayersCount)/10 + 1

	reqBody := map[string]any{
		"multiplier": multiplier,
		"answer":     answer,
		"pda_nr":     duel.RoomNumber,
		"payer":      userPrivateKey.PublicKey().String(),
	}
	instructions, err := s.GetInstructionsFromContractService(ctx, reqBody, "join", userPrivateKey)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(userPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	txHash, err := s.sendTxWithTracker(ctx,
		tx,
		txSignerPrivateKeyGetter(userPrivateKey))
	if err != nil {
		return "", apperrors.ServiceUnavailable("failed to send transaction", err)
	}

	return txHash.String(), nil
}

func (s *WalletService) RewardDuelWinners(
	ctx context.Context,
	usdWinAmount uint64,
	winners []model.CryptoDuelPlayer,
) ([]string, error) {
	if len(winners) == 0 {
		return []string{}, nil
	}

	adminTokenAccount, _, err := solana.FindAssociatedTokenAddress(s.adminPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil || adminTokenAccount == ZeroValuePublicKey {
		return nil, apperrors.Internal("failed to find associated token account for user rewarding", err)
	}

	allTransferInstructions, err := getTransferInstruction(s.adminPrivateKey, usdWinAmount, winners)
	if err != nil {
		return nil, err
	}

	separatedInstructions := separateInstructions(allTransferInstructions)

	txHashes := make([]string, 0, len(separatedInstructions))

	for _, instructions := range separatedInstructions {
		tx, err := s.NewTransactionForSimulation(
			instructions,
			txSignerPrivateKeyGetter(s.adminPrivateKey),
			solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
		if err != nil {
			return nil, err
		}

		computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
		if err != nil {
			if len(instructions) > 1 {
				computeUnits = FallBackCUTransfer * uint32(len(instructions))
			}
		}

		computeUnits = uint32(float64(computeUnits)*CUExtraCapacityCoefficient + 300)
		cuPriceInstruction, err := computebudget.NewSetComputeUnitPriceInstructionBuilder().
			SetMicroLamports(s.PriorityTracker.GetHighPriorityMicroLamports()).
			ValidateAndBuild()
		if err != nil {
			return nil, apperrors.Internal("failed to set transaction compute unit price", err)
		}

		cuLimitInstruction, err := computebudget.NewSetComputeUnitLimitInstructionBuilder().
			SetUnits(computeUnits).
			ValidateAndBuild()
		if err != nil {
			return nil, apperrors.Internal("failed to set transaction compute unit limit", err)
		}

		// Compute Unit Price and Compute Unit Limit instructions must be first
		instructions = append([]solana.Instruction{cuPriceInstruction, cuLimitInstruction}, instructions...)

		tx, err = solana.NewTransaction(
			instructions,
			solana.Hash{},
			solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
		if err != nil {
			return nil, apperrors.Internal("failed to create transaction", err)
		}

		txHash, err := s.sendTransaction(
			ctx,
			tx,
			txSignerPrivateKeyGetter(s.adminPrivateKey))
		if err != nil {
			return nil, err
		}

		txHashes = append(txHashes, txHash.String())
	}

	return txHashes, nil
}

func (s *WalletService) RewardDuelOwnerWithCommission(
	ctx context.Context,
	publicAddress string,
	commissionReward uint64,
) (string, error) {
	if commissionReward == 0 {
		return "", nil
	}

	adminTokenAccount, _, err := solana.FindAssociatedTokenAddress(s.adminPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil || adminTokenAccount == ZeroValuePublicKey {
		return "", apperrors.Internal("failed to find associated token account for user rewarding", err)
	}

	duelOwnerAddress, err := solana.PublicKeyFromBase58(publicAddress)
	if err != nil || duelOwnerAddress == ZeroValuePublicKey {
		return "", apperrors.BadRequest("recipient is not valid solana address", err)
	}

	duelOwnerATA, _, err := solana.FindAssociatedTokenAddress(duelOwnerAddress, USDCMintAddress)
	if err != nil || duelOwnerATA == ZeroValuePublicKey {
		return "", apperrors.BadRequest("failed to find recipient associated token account", err)
	}

	info, err := s.SolanaRPC.GetAccountInfo(ctx, duelOwnerATA)
	if err != nil && !errors.Is(err, rpc.ErrNotFound) {
		return "", apperrors.ServiceUnavailable("failed to get recipient's account info", err)
	}

	inst := make([]solana.Instruction, 0, 2)
	if info == nil || info.Value == nil || info.Value.Owner == ZeroValuePublicKey {
		initTokenAccountInstruction, err := associatedtokenaccount.NewCreateInstruction(
			s.adminPrivateKey.PublicKey(),
			duelOwnerAddress,
			USDCMintAddress).ValidateAndBuild()
		if err != nil {
			return "", apperrors.Internal("failed to build token account initialization instruction", err)
		}

		inst = append(inst, initTokenAccountInstruction)
	}

	transferInstruction, err := token.NewTransferInstruction(
		commissionReward,
		adminTokenAccount,
		duelOwnerATA,
		s.adminPrivateKey.PublicKey(),
		[]solana.PublicKey{s.adminPrivateKey.PublicKey()}).ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to build transfer transaction", err)
	}

	inst = append(inst, transferInstruction)

	txHash, err := s.SendTransaction(ctx, inst)
	if err != nil {
		return "", err
	}

	return txHash, nil
}

func (s *WalletService) SendTransaction(
	ctx context.Context,
	instructions []solana.Instruction,
) (string, error) {
	tx, err := s.NewTransactionForSimulation(
		instructions,
		txSignerPrivateKeyGetter(s.adminPrivateKey),
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
	if err != nil {
		return "", err
	}

	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil {
		if len(instructions) > 1 {
			computeUnits = FallBackCUTransfer * uint32(len(instructions))
		}
	}

	computeUnits = uint32(float64(computeUnits)*CUExtraCapacityCoefficient + 300)
	cuPriceInstruction, err := computebudget.NewSetComputeUnitPriceInstructionBuilder().
		SetMicroLamports(s.PriorityTracker.GetHighPriorityMicroLamports()).
		ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to set transaction compute unit price", err)
	}

	cuLimitInstruction, err := computebudget.NewSetComputeUnitLimitInstructionBuilder().
		SetUnits(computeUnits).
		ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to set transaction compute unit limit", err)
	}

	// Compute Unit Price and Compute Unit Limit instructions must be first
	instructions = append([]solana.Instruction{cuPriceInstruction, cuLimitInstruction}, instructions...)

	tx, err = solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	txHash, err := s.sendTransaction(
		ctx,
		tx,
		txSignerPrivateKeyGetter(s.adminPrivateKey))
	if err != nil {
		return "", err
	}

	return txHash.String(), nil
}

func (s *WalletService) TransferUSDCBulk(
	ctx context.Context,
	amount uint64,
	players []model.CryptoDuelPlayer,
) ([]string, error) {
	adminTokenAccount, _, err := solana.FindAssociatedTokenAddress(s.adminPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil || adminTokenAccount == ZeroValuePublicKey {
		return nil, apperrors.Internal("failed to find associated token account for user rewarding", err)
	}

	allTransferInstructions, err := getTransferInstruction(s.adminPrivateKey, amount, players)
	if err != nil {
		return nil, err
	}

	separatedInstructions := separateInstructions(allTransferInstructions)
	txHashes := make([]string, 0, len(separatedInstructions))

	for _, instructions := range separatedInstructions {
		tx, err := s.NewTransactionForSimulation(
			instructions,
			txSignerPrivateKeyGetter(s.adminPrivateKey),
			solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
		if err != nil {
			return nil, err
		}

		computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
		if err != nil {
			if len(instructions) > 1 {
				computeUnits = FallBackCUTransfer * uint32(len(instructions))
			}
		}

		computeUnits = uint32(float64(computeUnits)*CUExtraCapacityCoefficient + 300)
		cuPriceInstruction, err := computebudget.NewSetComputeUnitPriceInstructionBuilder().
			SetMicroLamports(s.PriorityTracker.GetHighPriorityMicroLamports()).
			ValidateAndBuild()
		if err != nil {
			return nil, apperrors.Internal("failed to set transaction compute unit price", err)
		}

		cuLimitInstruction, err := computebudget.NewSetComputeUnitLimitInstructionBuilder().
			SetUnits(computeUnits).
			ValidateAndBuild()
		if err != nil {
			return nil, apperrors.Internal("failed to set transaction compute unit limit", err)
		}

		// Compute Unit Price and Compute Unit Limit instructions must be first
		instructions = append([]solana.Instruction{cuPriceInstruction, cuLimitInstruction}, instructions...)

		tx, err = solana.NewTransaction(
			instructions,
			solana.Hash{},
			solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
		if err != nil {
			return nil, apperrors.Internal("failed to create transaction", err)
		}

		txHash, err := s.sendTransaction(
			ctx,
			tx,
			txSignerPrivateKeyGetter(s.adminPrivateKey))
		if err != nil {
			return nil, err
		}

		txHashes = append(txHashes, txHash.String())
	}

	return txHashes, nil
}

func getTransferInstruction(
	sender solana.PrivateKey,
	amount uint64,
	players []model.CryptoDuelPlayer,
) ([]solana.Instruction, error) {
	senderTokenAccount, _, err := solana.FindAssociatedTokenAddress(sender.PublicKey(), USDCMintAddress)
	if err != nil || senderTokenAccount == ZeroValuePublicKey {
		return nil, apperrors.Internal("failed to find associated token account for user rewarding", err)
	}

	instructions := make([]solana.Instruction, 0, len(players)+2)

	for _, player := range players {
		recipient, err := solana.PublicKeyFromBase58(player.PublicAddress)
		if err != nil || recipient == ZeroValuePublicKey {
			return nil, apperrors.BadRequest("recipient is not valid solana address", err)
		}

		recipientTokenAccount, _, err := solana.FindAssociatedTokenAddress(recipient, USDCMintAddress)
		if err != nil || recipientTokenAccount == ZeroValuePublicKey {
			return nil, apperrors.BadRequest("failed to find recipient associated token account", err)
		}

		transferInstruction, err := token.NewTransferInstruction(
			amount,
			senderTokenAccount,
			recipientTokenAccount,
			sender.PublicKey(),
			[]solana.PublicKey{sender.PublicKey()}).ValidateAndBuild()
		if err != nil {
			return nil, apperrors.Internal("failed to build transfer transaction", err)
		}

		instructions = append(instructions, transferInstruction)
	}

	return instructions, nil
}

const TransferInstructionsPerTransaction = 32

func separateInstructions(instructions []solana.Instruction) [][]solana.Instruction {
	separatedInstructions := make([][]solana.Instruction, 0, len(instructions)/TransferInstructionsPerTransaction+1)

	for i := 0; i < len(instructions); i += TransferInstructionsPerTransaction {
		sliceEnd := min(i+TransferInstructionsPerTransaction, len(instructions))
		separatedInstructions = append(separatedInstructions, instructions[i:sliceEnd])
	}

	return separatedInstructions
}

func (s *WalletService) CloseSolanaRoom(
	ctx context.Context,
	duelRoomNumber uint64,
) (string, error) {
	adminTokenAccount, _, err := solana.FindAssociatedTokenAddress(s.adminPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil || adminTokenAccount == ZeroValuePublicKey {
		return "", apperrors.Internal("failed to find associated token account for user rewarding", err)
	}

	reqBody := map[string]any{
		"pda_nr": duelRoomNumber,
	}
	instructions, err := s.GetInstructionsFromContractService(
		ctx,
		reqBody,
		"close",
		s.adminPrivateKey)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(s.adminPrivateKey.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	txHash, err := s.sendTxWithTracker(
		ctx,
		tx,
		txSignerPrivateKeyGetter(s.adminPrivateKey))
	if err != nil {
		return "", err
	}

	return txHash.String(), nil
}

func findProgramAddress(seed1, seed2 []byte, programID solana.PublicKey) (solana.PublicKey, uint8) {
	seed := [][]byte{seed1, seed2}
	pda, bump, err := solana.FindProgramAddress(seed, programID)
	if err != nil {
		return solana.PublicKey{}, 0
	}

	return pda, bump
}

func uint32ToBytesLE(value uint32) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, value)

	return buf.Bytes()
}

func (s *WalletService) GetPDAByRoomNumber(num uint32) (solana.PublicKey, error) {
	programID, err := solana.PublicKeyFromBase58(s.contractAddress)
	if err != nil {
		return solana.PublicKey{}, apperrors.Internal("failed to get contract public address", err)
	}

	seed1 := []byte("data")
	seed2 := uint32ToBytesLE(num)

	pda, _ := findProgramAddress(seed1, seed2, programID)

	return pda, nil
}

type CompiledInstruction struct {
	programID solana.PublicKey
	accounts  []*solana.AccountMeta
	data      []byte
}

func decompileInstruction(instruction solana.CompiledInstruction, tx *solana.Transaction) (*CompiledInstruction, error) {
	programID, err := tx.ResolveProgramIDIndex(instruction.ProgramIDIndex)
	if err != nil {
		return nil, fmt.Errorf("resolve program ID: %w", err)
	}

	accounts, err := instruction.ResolveInstructionAccounts(&tx.Message)
	if err != nil {
		return nil, fmt.Errorf("resolve instruction account: %w", err)
	}

	return &CompiledInstruction{
		programID: programID,
		accounts:  accounts,
		data:      instruction.Data,
	}, nil
}

func (c *CompiledInstruction) ProgramID() solana.PublicKey {
	return c.programID
}

func (c *CompiledInstruction) Accounts() []*solana.AccountMeta {
	return c.accounts
}

func (c *CompiledInstruction) Data() ([]byte, error) {
	return c.data, nil
}

var ComputeBudgetProgramID = solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")

func RemoveComputeBudgetInstructionsFromTx(tx *solana.Transaction) ([]solana.Instruction, error) {
	instructions := make([]solana.Instruction, 0, len(tx.Message.Instructions))

	for _, instruction := range tx.Message.Instructions {
		instr, err := decompileInstruction(instruction, tx)
		if err != nil {
			return nil, apperrors.Internal("failed to decompile instruction", err)
		}

		if instr.ProgramID() == ComputeBudgetProgramID {
			continue
		}

		instructions = append(instructions, instr)
	}

	return instructions, nil
}

func ExtractTxFromResp(resp *resty.Response) (*solana.Transaction, error) {
	if resp == nil || !resp.IsSuccess() {
		return nil, apperrors.ServiceUnavailable("failed to get transaction from contract service: resp is nil")
	}

	var respData model.ContractServiceTxResp
	if err := json.Unmarshal(resp.Body(), &respData); err != nil {
		return nil, apperrors.ServiceUnavailable("failed to unmarshal raw transaction", err)
	}

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(respData.RawTx))
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to decode a transaction", err)
	}

	return tx, nil
}

func (s *WalletService) GetInstructionsFromContractService(
	ctx context.Context,
	reqBody map[string]any,
	endpoint string,
	signer solana.PrivateKey,
) ([]solana.Instruction, error) {
	tx, err := s.GetTxFromContractService(reqBody, endpoint)
	if err != nil {
		return nil, err
	}

	recentBlockHashResp, err := s.SolanaRPC.GetLatestBlockhash(ctx, Finalized)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get latest block hash", err)
	}

	tx.Message.RecentBlockhash = recentBlockHashResp.Value.Blockhash

	_, err = tx.Sign(txSignerPrivateKeyGetter(signer))
	if err != nil {
		return nil, apperrors.Internal("failed to sign a transaction", err)
	}

	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil {
		return nil, err
	}

	computeUnits = uint32(float64(computeUnits) * CUExtraCapacityCoefficient)
	cuPriceInstruction, err := computebudget.NewSetComputeUnitPriceInstructionBuilder().
		SetMicroLamports(s.PriorityTracker.GetMediumPriorityMicroLamports()).
		ValidateAndBuild()
	if err != nil {
		return nil, apperrors.Internal("failed to set transaction compute unit price", err)
	}

	cuLimitInstruction, err := computebudget.NewSetComputeUnitLimitInstructionBuilder().
		SetUnits(computeUnits).
		ValidateAndBuild()
	if err != nil {
		return nil, apperrors.Internal("failed to set transaction compute unit limit", err)
	}

	instructions := make([]solana.Instruction, 0, 3)
	instructions = append(instructions, cuPriceInstruction, cuLimitInstruction)

	for _, instruction := range tx.Message.Instructions {
		inst, err := decompileInstruction(instruction, tx)
		if err != nil {
			return nil, apperrors.ServiceUnavailable("failed to decompile instruction", err)
		}

		instructions = append(instructions, inst)
	}

	return instructions, nil
}

func (s *WalletService) GetTxFromContractService(
	reqBody map[string]any,
	endpoint string,
) (*solana.Transaction, error) {
	resp, err := s.HTTPClient.R().
		SetBody(reqBody).
		SetHeader("Content-Type", "application/json").
		Put(s.contractAddressAPI + endpoint)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to get transaction from contract service", err)
	}

	tx, err := ExtractTxFromResp(resp)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func GetTxInstructions(txs ...*solana.Transaction) ([]solana.Instruction, error) {
	instructionsCount := 0
	for _, tx := range txs {
		if tx == nil {
			continue
		}

		instructionsCount += len(tx.Message.Instructions)
	}

	instructions := make([]solana.Instruction, 0, instructionsCount)

	for _, tx := range txs {
		if tx == nil {
			continue
		}

		for _, instruction := range tx.Message.Instructions {
			inst, err := decompileInstruction(instruction, tx)
			if err != nil {
				return nil, apperrors.ServiceUnavailable("failed to decompile instruction", err)
			}

			instructions = append(instructions, inst)
		}
	}

	return instructions, nil
}
