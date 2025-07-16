package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gagliardetto/solana-go"
	lookup "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"gitlab.com/duel-duck/duel-duck-api/internal/client/jupiter"
	sol "gitlab.com/duel-duck/duel-duck-api/internal/client/solana"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	repo "gitlab.com/duel-duck/duel-duck-api/pkg/repository"
	"go.uber.org/zap"
)

// TODO: Move variables to .env and config

var (
	USDCMintAddress = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	//USDCMintAddress = solana.MustPublicKeyFromBase58("Gh9ZwEmdLJ8DscKNTkTqPbNwLNNBjuSzaG9Vp2KGtKJr")
)

const (
	USDCMintDecimals                uint8  = 6
	USDCMintDecimalsMultiplier      uint64 = 1_000_000 // 10^6
	USDCMintDecimalsMultiplierFloat        = float64(USDCMintDecimalsMultiplier)

	SolMintDecimals uint8 = 9
)

type proceedTransferData struct {
	SenderAccount         solana.PrivateKey
	SenderTokenAddress    solana.PublicKey
	RecipientAddress      solana.PublicKey
	RecipientTokenAddress solana.PublicKey
	Mint                  solana.PublicKey
	Amount                uint64
	Decimals              uint8
}

func newProceedTransferData(
	sender solana.PrivateKey,
	recipient solana.PublicKey,
	mint solana.PublicKey,
	amount uint64,
	decimals uint8,
) (*proceedTransferData, error) {
	senderTokenAddress, _, err := solana.FindAssociatedTokenAddress(sender.PublicKey(), USDCMintAddress)
	if err != nil {
		return nil, apperrors.BadRequest("failed to get sender's associated token account", err)
	}

	recipientTokenAddress, _, err := solana.FindAssociatedTokenAddress(recipient, USDCMintAddress)
	if err != nil {
		return nil, apperrors.BadRequest("failed to get recipient's associated token account", err)
	}

	return &proceedTransferData{
		SenderAccount:         sender,
		SenderTokenAddress:    senderTokenAddress,
		RecipientAddress:      recipient,
		RecipientTokenAddress: recipientTokenAddress,
		Mint:                  mint,
		Amount:                amount,
		Decimals:              decimals,
	}, nil
}

var ZeroValuePublicKey solana.PublicKey

func (s *WalletService) Transfer(
	ctx context.Context,
	senderID uuid.UUID,
	recipientPublicKeyBase58 string,
	amount uint64,
) (string, error) {
	recipient, err := solana.PublicKeyFromBase58(recipientPublicKeyBase58)
	if err != nil || recipient == ZeroValuePublicKey {
		return "", apperrors.BadRequest("recipient is not valid solana address")
	}

	if err = s.senderExists(ctx, senderID); err != nil {
		return "", err
	}

	senderPrivateKeyBase58, err := s.privateKeyRepository.GetPrivateKeyBase58(ctx, senderID)
	if err != nil {
		return "", err
	}

	sender, err := solana.PrivateKeyFromBase58(senderPrivateKeyBase58)
	if err != nil {
		return "", apperrors.Internal("private key assigned to sender is not valid solana address", err)
	}

	data, err := newProceedTransferData(
		sender,
		recipient,
		USDCMintAddress,
		amount,
		USDCMintDecimals)
	if err != nil || data == nil {
		return "", apperrors.BadRequest("failed to prepare transaction data", err)
	}

	hasEnoughBalance, err := s.HasEnoughTokenBalance(ctx, data.SenderTokenAddress, amount)
	if err != nil {
		return "", err
	}

	if !hasEnoughBalance {
		return "", apperrors.BadRequest("not enough balance to proceed a transaction")
	}

	// custom context instead of fiber context, so client isn't able to interrupt transaction
	ctx = context.Background()

	txHash, err := s.proceedTransfer(ctx, data)
	if err != nil {
		return "", err
	}

	return txHash, nil
}

func (s *WalletService) senderExists(ctx context.Context, senderID uuid.UUID) error {
	ok, err := s.UserRepository.Exists(ctx, &model.User{ID: senderID})
	if err != nil {
		return apperrors.Internal("failed to get sender by id", err)
	}

	if !ok {
		return apperrors.BadRequest("user with id does not exist", nil)
	}

	return nil
}

const (
	Finalized  = rpc.CommitmentFinalized
	Commitment = rpc.CommitmentConfirmed
	Processed  = rpc.CommitmentProcessed
)

func (s *WalletService) GetTokenBalance(
	ctx context.Context,
	ata solana.PublicKey,
) (uint64, error) {
	balance, err := s.SolanaRPC.GetTokenAccountBalance(ctx, ata, Commitment)
	if err != nil || balance == nil || balance.Value == nil {
		return 0, apperrors.ServiceUnavailable("failed to get ata balance", err)
	}

	balanceAmount, err := strconv.ParseUint(balance.Value.Amount, 10, 64)
	if err != nil {
		return 0, apperrors.ServiceUnavailable("failed to parse token balance amount", err)
	}

	return balanceAmount, nil
}

func (s *WalletService) HasEnoughTokenBalance(
	ctx context.Context,
	ata solana.PublicKey,
	requiredAmount uint64,
) (bool, error) {
	tokenBalance, err := s.GetTokenBalance(ctx, ata)
	if err != nil {
		return false, err
	}

	fmt.Println("current token balance:", tokenBalance)

	return tokenBalance >= requiredAmount, nil
}

func (s *WalletService) GetSolBalance(
	ctx context.Context,
	pk solana.PublicKey,
) (uint64, error) {
	balance, err := s.SolanaRPC.GetBalance(ctx, pk, Commitment)
	if err != nil || balance == nil {
		return 0, apperrors.ServiceUnavailable("failed to get ata balance", err)
	}

	return balance.Value, nil
}

func (s *WalletService) HasEnoughSolBalance(
	ctx context.Context,
	pk solana.PublicKey,
	requiredAmount uint64,
) (bool, error) {
	balance, err := s.GetSolBalance(ctx, pk)
	if err != nil {
		return false, err
	}

	if balance < requiredAmount {
		return false, nil
	}

	return true, nil
}

const (
	// TransferTransactionInstructionsCount
	// Fist two are Compute Unit Price and Compute Unit Limit Instructions
	// Recipient token account initialization instruction if needed
	// And token transfer instruction
	TransferTransactionInstructionsCount = 4

	FallBackCUTransfer = 5026

	// FallBackCUTransferChecked and FallBackCUTransferWithTokenAccountInit are average values
	// we got by simulating transactions
	FallBackCUTransferChecked              = 6254
	FallBackCUTransferWithTokenAccountInit = 30195

	// CUExtraCapacityCoefficient sometimes transaction
	// may consume a bit more Compute Units then usual
	CUExtraCapacityCoefficient = 1.05
)

func (s *WalletService) proceedTransfer(ctx context.Context, data *proceedTransferData) (string, error) {
	instructions := make([]solana.Instruction, 0, TransferTransactionInstructionsCount)

	info, err := s.SolanaRPC.GetAccountInfo(ctx, data.RecipientTokenAddress)
	if err != nil && !errors.Is(err, rpc.ErrNotFound) {
		return "", apperrors.ServiceUnavailable("failed to get recipient's account info", err)
	}

	if info == nil || info.Value == nil || info.Value.Owner == ZeroValuePublicKey {
		initTokenAccountInstruction, err := associatedtokenaccount.NewCreateInstruction(
			data.SenderAccount.PublicKey(),
			data.RecipientAddress,
			USDCMintAddress).ValidateAndBuild()
		if err != nil {
			return "", apperrors.Internal("failed to build token account initialization instruction", err)
		}

		instructions = append(instructions, initTokenAccountInstruction)
	}

	transferInstruction, err := token.NewTransferCheckedInstruction(
		data.Amount,
		USDCMintDecimals,
		data.SenderTokenAddress,
		USDCMintAddress,
		data.RecipientTokenAddress,
		data.SenderAccount.PublicKey(),
		[]solana.PublicKey{data.SenderAccount.PublicKey()}).ValidateAndBuild()
	if err != nil {
		return "", apperrors.Internal("failed to build token transfer instruction", err)
	}

	instructions = append(instructions, transferInstruction)

	tx, err := s.NewTransactionForSimulation(
		instructions,
		txSignerPrivateKeyGetter(data.SenderAccount),
		solana.TransactionPayer(data.SenderAccount.PublicKey()))
	if err != nil {
		return "", err
	}

	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil {
		if len(instructions) > 1 {
			computeUnits = FallBackCUTransferWithTokenAccountInit
		} else {
			computeUnits = FallBackCUTransferChecked
		}
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

	// Compute Unit Price and Compute Unit Limit instructions must be first
	instructions = append([]solana.Instruction{cuPriceInstruction, cuLimitInstruction}, instructions...)

	tx, err = solana.NewTransaction(
		instructions,
		solana.Hash{},
		solana.TransactionPayer(data.SenderAccount.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to create transaction", err)
	}

	sig, err := s.sendTxWithTracker(
		ctx,
		tx,
		txSignerPrivateKeyGetter(data.SenderAccount))
	if err != nil {
		return "", err
	}

	return sig.String(), nil
}

var (
	TransactionMaxRetryCount uint = 10
)

func (s *WalletService) sendTransaction(
	ctx context.Context,
	tx *solana.Transaction,
	privateKeyGetter func(key solana.PublicKey) *solana.PrivateKey,
) (solana.Signature, error) {
	opts := rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: Finalized,
		MaxRetries:          &TransactionMaxRetryCount,
	}

	if err := s.RefreshBlockHash(ctx, &tx.Message); err != nil {
		return solana.Signature{}, err
	}

	_, err := tx.Sign(privateKeyGetter)
	if err != nil {
		return solana.Signature{}, apperrors.Internal("failed to sign transaction", err)
	}

	sig, err := s.SolanaRPC.SendTransactionWithOpts(ctx, tx, opts)
	if err != nil {
		return solana.Signature{}, apperrors.Internal("failed to send transaction", err)
	}

	return sig, nil
}

func (s *WalletService) RefreshBlockHash(ctx context.Context, txMessage *solana.Message) error {
	recentBlockHashResp, err := s.SolanaRPC.GetLatestBlockhash(ctx, Finalized)
	if err != nil {
		return apperrors.ServiceUnavailable("failed to get latest block hash", err)
	}

	txMessage.RecentBlockhash = recentBlockHashResp.Value.Blockhash
	return nil
}

func (s *WalletService) GetSimulationComputeUnits(
	ctx context.Context,
	tx *solana.Transaction,
) (uint32, error) {
	opts := &rpc.SimulateTransactionOpts{
		ReplaceRecentBlockhash: true,
		SigVerify:              false, // conflicts with ReplaceRecentBlockhash
	}

	result, err := s.simulateTransaction(ctx, tx, opts)
	if err != nil {
		return 0, err
	}

	if result == nil {
		return 0, apperrors.Internal("transaction simulation: tx value is nil")
	}

	if result.Err != nil {
		zap.L().Error("transaction simulation", zap.Any("err", result.Err))
		return 0, sol.ParseLogsForError(result.Logs)
	}

	if result.UnitsConsumed == nil {
		return 0, apperrors.Internal("transaction simulation: 0 units consumed")
	}

	return uint32(*result.UnitsConsumed), nil
}

func (s *WalletService) simulateTransaction(
	ctx context.Context,
	tx *solana.Transaction,
	opts *rpc.SimulateTransactionOpts,
) (*rpc.SimulateTransactionResult, error) {
	sTx, err := s.SolanaRPC.SimulateTransactionWithOpts(ctx, tx, opts)
	if err != nil {
		return nil, apperrors.ServiceUnavailable("failed to send transaction on simulation", err)
	}

	if sTx == nil {
		return nil, apperrors.ServiceUnavailable("failed to get transaction compute units, tx is nil", err)
	}

	return sTx.Value, nil
}

func txSignerPrivateKeyGetter(sender solana.PrivateKey) func(solana.PublicKey) *solana.PrivateKey {
	return func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(sender.PublicKey()) {
			return &sender
		}

		return nil
	}
}

func txTwoSignersPrivateKeyGetter(admin, user solana.PrivateKey) func(solana.PublicKey) *solana.PrivateKey {
	return func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(admin.PublicKey()) {
			return &admin
		}

		if key.Equals(user.PublicKey()) {
			return &user
		}

		return nil
	}
}

func (s *WalletService) TransferDuckPoints(
	ctx context.Context,
	senderID uuid.UUID,
	recipientAddress string,
	amount uint64,
) (err error) {
	recipient, err := s.UserRepository.FindByPublicAddress(ctx, recipientAddress)
	if err != nil {
		return apperrors.NotFound("failed to find recipient by public address", err)
	}

	if recipient == nil {
		return apperrors.NotFound("recipient not found by public address", err)
	}

	sender, err := s.UserRepository.GetByID(ctx, senderID)
	if err != nil {
		return apperrors.Internal("failed to get user by id", err)
	}

	if sender.Balance.LessThan(amount) {
		return apperrors.BadRequest("not enough duck points to transfer")
	}

	err = s.TransactionManager.WithinTransaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		_, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, senderID, -int(amount))
		if err != nil {
			return err
		}

		_, err = s.UserRepository.WithTx(tx).AlterBalanceByID(ctx, recipient.ID, int(amount))
		if err != nil {
			return err
		}

		transferTx := model.NewDuckPointsTransaction(sender.PublicAddress, recipientAddress, amount)
		err = s.UserRepository.WithTx(tx).AddDuckPointsTransactionToHistory(ctx, transferTx)

		return err
	})
	if err != nil {
		return apperrors.BadRequest("failed transfer duck points", err)
	}

	return nil
}

func (s *WalletService) NewTransactionForSimulation(
	instructions []solana.Instruction,
	privateKeyGetter func(key solana.PublicKey) *solana.PrivateKey,
	opts ...solana.TransactionOption,
) (*solana.Transaction, error) {
	tx, err := solana.NewTransaction(
		instructions,
		solana.Hash{}, // latest block hash will be set just before transaction sending or transaction simulation
		opts...)
	if err != nil {
		return nil, apperrors.Internal("failed to create transaction", err)
	}

	_, err = tx.Sign(privateKeyGetter)
	if err != nil {
		return nil, apperrors.Internal("failed to sign a transaction", err)
	}

	return tx, nil
}

func (s *WalletService) DuckPointsTransferHistory(
	ctx context.Context,
	userID uuid.UUID,
	opts repo.Options,
) ([]model.DuckPointsTransaction, error) {
	user, err := s.UserRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to find user by id", err)
	}

	transactions, err := s.UserRepository.GetDuckPointsTransferHistory(ctx, user.PublicAddress, opts)
	if err != nil {
		return nil, apperrors.Internal("failed to get transaction history", err)
	}

	return transactions, nil
}

const (
	// SwapSlippageBPS = 500 means that we are ready to lose up to 5% of the initial amount
	SwapSlippageBPS int64 = 500

	CoefficientToBPSRelation = 1000

	SwapMaxRetryCount = 4
)

func (s *WalletService) Swap(
	ctx context.Context,
	userID uuid.UUID,
	inputMintAddress string,
	outputMintAddress string,
	amount float64,
) (string, error) {
	inputMint, err := solana.PublicKeyFromBase58(inputMintAddress)
	if err != nil {
		return "", apperrors.NotFound("input mint not found", err)
	}

	outputMint, err := solana.PublicKeyFromBase58(outputMintAddress)
	if err != nil {
		return "", apperrors.NotFound("output mint not found", err)
	}

	userPrivateKey, err := s.privateKeyRepository.GetPrivateKeyBase58(ctx, userID)
	if err != nil {
		return "", apperrors.Internal("failed to find user by id", err)
	}

	privateKey, err := solana.PrivateKeyFromBase58(userPrivateKey)
	if err != nil {
		return "", apperrors.Internal("user has invalid public address", err)
	}

	inputATA, _, err := solana.FindAssociatedTokenAddress(privateKey.PublicKey(), inputMint)
	if err != nil {
		return "", apperrors.Internal("failed to find associated token address", err)
	}

	tokenInfo, err := s.GetTokenInfo(inputMint.String())
	if err != nil {
		return "", apperrors.BadRequest("mint not found", err)
	}

	decimals, err := strconv.ParseUint(tokenInfo.Decimals, 10, 64)
	if err != nil {
		return "", apperrors.BadRequest("failed to get input token decimals", err)
	}

	rawAmount := uint64(amount * math.Pow10(int(decimals)))

	if inputMint == solana.SolMint {
		hasEnough, err := s.HasEnoughSolBalance(ctx, privateKey.PublicKey(), rawAmount)
		if err != nil {
			return "", err
		}
		if !hasEnough {
			return "", apperrors.BadRequest("not enough balance to proceed a transaction")
		}
	} else {
		hasEnough, err := s.HasEnoughTokenBalance(ctx, inputATA, rawAmount)
		if err != nil {
			return "", err
		}
		if !hasEnough {
			return "", apperrors.BadRequest("not enough balance to proceed a transaction")
		}
	}

	var platformFeeBPS int64 = 0
	if inputMint == USDCMintAddress || outputMint == USDCMintAddress {
		platformFeeBPS = int64(s.SwapCommissionCoefficient * CoefficientToBPSRelation)
	}

	for range SwapMaxRetryCount {
		txHash, err := s.swap(
			ctx,
			privateKey,
			inputMintAddress,
			outputMintAddress,
			rawAmount,
			platformFeeBPS,
		)
		if err != nil {
			zap.L().Error(
				"failed to swap tokens",
				zap.String("user_id", userID.String()),
				zap.String("input_mint", inputMintAddress),
				zap.String("output_mint", outputMintAddress),
				zap.Error(err))
			continue
		}

		return txHash, nil
	}

	return "", apperrors.Internal("failed to perform token swap, no retries left")
}

func (s *WalletService) swap(
	ctx context.Context,
	signer solana.PrivateKey,
	inputMint string,
	outputMint string,
	rawAmount uint64,
	platformFeeBPS int64,
) (string, error) {
	data, err := s.Jupiter.GetQuote(ctx, jupiter.GetQuoteParams{
		InputMintAddress:  inputMint,
		OutputMintAddress: outputMint,
		Amount:            int64(rawAmount),
		SlippageBps:       SwapSlippageBPS,
		PlatformFeeBps:    platformFeeBPS,
	})
	if err != nil {
		return "", err
	}

	commissionsATA, _, err := solana.FindAssociatedTokenAddress(s.adminPrivateKey.PublicKey(), USDCMintAddress)
	if err != nil {
		return "", apperrors.Internal("failed to find admins associated token address", err)
	}

	out, qErr := s.Jupiter.GetSwapTransaction(ctx, *data, signer.PublicKey(), commissionsATA)
	if qErr != nil {
		return "", apperrors.ServiceUnavailable("failed to get resp from jupiter swap transaction", err)
	}

	if err = s.processTransactionWithAddressLookups(ctx, out.SwapTransaction); err != nil {
		return "", err
	}

	instructions, err := RemoveComputeBudgetInstructionsFromTx(out.SwapTransaction)
	if err != nil {
		return "", err
	}

	tx, err := s.NewTransactionForSimulation(
		instructions,
		txSignerPrivateKeyGetter(signer),
		solana.TransactionPayer(signer.PublicKey()))
	if err != nil {
		return "", err
	}

	ctx = context.Background()
	computeUnits, err := s.GetSimulationComputeUnits(ctx, tx)
	if err != nil {
		return "", err
	}

	computeUnits += uint32(float64(computeUnits) * 0.05)

	highPriorityMicroLamports := s.PriorityTracker.GetMediumPriorityMicroLamports()

	instructions = append(instructions,
		computebudget.NewSetComputeUnitPriceInstructionBuilder().
			SetMicroLamports(highPriorityMicroLamports).
			Build(),
		computebudget.NewSetComputeUnitLimitInstructionBuilder().
			SetUnits(computeUnits).
			Build(),
	)

	tx, err = solana.NewTransaction(
		instructions,
		out.SwapTransaction.Message.RecentBlockhash,
		solana.TransactionPayer(signer.PublicKey()))
	if err != nil {
		return "", apperrors.Internal("failed to build transaction", err)
	}

	_, err = tx.Sign(txSignerPrivateKeyGetter(signer))
	if err != nil {
		return "", apperrors.Internal("failed to sign transaction", err)
	}

	_, err = s.SolanaRPC.SimulateTransaction(ctx, tx)
	if err != nil {
		return "", apperrors.Internal("failed to simulate transaction", err)
	}

	sig, err := s.sendTxWithTracker(ctx, tx, txSignerPrivateKeyGetter(signer))
	if err != nil {
		return "", err
	}

	return sig.String(), nil
}

func (s *WalletService) GetTokenInfo(address string) (model.TokenInfo, error) {
	if address == "" {
		return model.TokenInfo{}, fmt.Errorf("address is required")
	}

	const TokenURL = "https://solana-gateway.moralis.io/token/mainnet/%s/metadata"

	finalUrl := fmt.Sprintf(TokenURL, address)
	req, _ := http.NewRequest("GET", finalUrl, nil)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-API-Key", s.moralisAPIKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return model.TokenInfo{}, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return model.TokenInfo{}, fmt.Errorf("cannot fetch info")
	}

	var info model.TokenInfo
	err = json.NewDecoder(res.Body).Decode(&info)
	if err != nil {
		return model.TokenInfo{}, err
	}

	return info, nil
}

func (s *WalletService) processTransactionWithAddressLookups(
	ctx context.Context,
	txx *solana.Transaction,
) error {
	if !txx.Message.IsVersioned() {
		return apperrors.ServiceUnavailable("invalid tx: only versioned transactions can contain lookups")
	}

	tblKeys := txx.Message.GetAddressTableLookups().GetTableIDs()
	if len(tblKeys) == 0 {
		return apperrors.ServiceUnavailable("no lookup tables in versioned tx")
	}

	numLookups := txx.Message.GetAddressTableLookups().NumLookups()
	if numLookups == 0 {
		return apperrors.ServiceUnavailable("no lookups in versioned transaction")
	}

	resolutions := make(map[solana.PublicKey]solana.PublicKeySlice, len(tblKeys))

	for _, key := range tblKeys {
		info, err := s.SolanaRPC.GetAccountInfo(ctx, key)
		if err != nil {
			return apperrors.ServiceUnavailable("failed to get account info", err)
		}

		tableContent, err := lookup.DecodeAddressLookupTableState(info.GetBinary())
		if err != nil {
			return apperrors.Internal("failed to decode address lookup table state", err)
		}

		resolutions[key] = tableContent.Addresses
	}

	err := txx.Message.SetAddressTables(resolutions)
	if err != nil {
		return apperrors.Internal("failed to set address tables", err)
	}

	err = txx.Message.ResolveLookups()
	if err != nil {
		return apperrors.Internal("failed to resolve lookups", err)
	}

	return nil
}

const TxConfirmationTimeout = time.Second * 40

func (s *WalletService) sendTxWithTracker(
	ctx context.Context,
	tx *solana.Transaction,
	privateKeyGetter func(key solana.PublicKey) *solana.PrivateKey,
) (solana.Signature, error) {
	sig, err := s.sendTransaction(ctx, tx, privateKeyGetter)
	if err != nil {
		return solana.Signature{}, err
	}

	sent, err := s.SigTracker.SubscribeForSignatureStatus(sig, TxConfirmationTimeout)
	if err != nil && !errors.Is(err, rpc.ErrNotConfirmed) {
		return solana.Signature{}, apperrors.Internal("subscribe to signature status", err)
	}

	if !sent {
		return solana.Signature{}, apperrors.Internal("tx was not confirmed: " + sig.String())
	}

	return sig, nil
}
