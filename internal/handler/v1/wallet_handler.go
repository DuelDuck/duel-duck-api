package v1

import (
	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
)

type WalletHandler struct {
	WalletService *service.WalletService
	JWTService    *service.JWTService
}

func NewWalletHandler(
	walletService *service.WalletService,
	jwtService *service.JWTService,
) *WalletHandler {
	return &WalletHandler{
		WalletService: walletService,
		JWTService:    jwtService,
	}
}

func (h *WalletHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	walletGroup := app.Group("/wallet")

	walletGroup.Use(auth.AuthMiddleware)
	{
		walletGroup.Post("/show", h.ShowWallet)

		walletGroup.Post("/transfer", h.Transfer)
		walletGroup.Post("/transfer-duck-points", h.TransferDuckPoints)

		walletGroup.Get("/duck-points-transfer-history", h.DuckPointsTransferHistory)

		walletGroup.Post("/swap", h.SwapCurrencies)

		walletGroup.Get("/notifications", h.GetAllTxNotifications)
		walletGroup.Delete("/notifications", h.DeleteTxNotifications)
	}

	tokenGroup := walletGroup.Group("/token")
	{
		tokenGroup.Get("/", h.GetUserWalletTokens)
		tokenGroup.Post("/", h.AddTokenToUserWallet)
		tokenGroup.Put("/", h.EditUserWalletToken)
		tokenGroup.Delete("/:mint", h.RemoveTokenFromUserWallet)
	}
}

func (h *WalletHandler) ShowWallet(c fiber.Ctx) error {
	var req model.ShowWalletReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	encryptedWalletData, err := h.WalletService.ShowWallet(c.Context(), claims.UserID, req.PublicCypherKey)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"wallet_data": encryptedWalletData,
	})
}

// Transfer godoc
//
//	@Summary		Transfer tokens to another user
//	@Description	Transfers USDC from the authenticated user's wallet to a recipient's wallet address.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.TransferReq		true	"Transfer request payload"
//	@Success		200				{object}	object{tx_hash=string}	"Transaction hash of the successful transfer"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Recipient not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Failure		503				{object}	apperrors.ErrorPublic	"External service unavailable"
//	@Router			/wallet/transfer [post]
func (h *WalletHandler) Transfer(c fiber.Ctx) error {
	var req model.TransferReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	txHash, err := h.WalletService.Transfer(c.Context(), claims.UserID, req.Recipient, req.Amount)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"tx_hash": txHash,
	})
}

// TransferDuckPoints godoc
//
//	@Summary		Transfer Duck Points to another user
//	@Description	Transfers Duck Points from the authenticated user's account to a recipient's DuelDuck account by public address.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.TransferReq		true	"Transfer request payload"
//	@Success		200				{object}	nil						"Duck Points transferred successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Recipient not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/wallet/transfer-duck-points [post]
func (h *WalletHandler) TransferDuckPoints(c fiber.Ctx) error {
	var req model.TransferReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.WalletService.TransferDuckPoints(c.Context(), claims.UserID, req.Recipient, req.Amount)
	if err != nil {
		return err
	}

	return nil
}

// DuckPointsTransferHistory godoc
//
//	@Summary		Get Duck Points transfer history
//	@Description	Retrieves the Duck Points transfer transaction history for the authenticated user, supporting advanced filtering, ordering, and pagination via query parameters.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization				header		string						true	"Authorization Bearer token"
//	@Param			opts.pagination.page_size	query		uint64						false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64						false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string						false	"Field to order by"
//	@Param			opts.order.order_type		query		string						false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string						false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string						false	"Filter operator"
//	@Param			opts.filters[0].value		query		string						false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool						false	"Filter OR condition"
//	@Success		200							{array}		model.DuckPointsTransaction	"List of Duck Points transfer transactions"
//	@Failure		400							{object}	apperrors.ErrorPublic		"Invalid request params or validation errors"
//	@Failure		401							{object}	apperrors.ErrorPublic		"Authentication required or invalid credentials"
//	@Failure		500							{object}	apperrors.ErrorPublic		"Internal server error"
//	@Router			/wallet/duck-points-transfer-history [get]
func (h *WalletHandler) DuckPointsTransferHistory(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	transactions, err := h.WalletService.DuckPointsTransferHistory(c.Context(), claims.UserID, req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(transactions)
}

// SwapCurrencies godoc
//
//	@Summary		Swap tokens between two currencies in the user's wallet
//	@Description	Swaps a specified amount of one token (input mint) for another (output mint) using the user's wallet. Returns the transaction hash of the swap operation.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.SwapReq			true	"Swap request payload containing input mint, output mint, and amount"
//	@Success		200				{object}	object{tx_hash=string}	"Swap transaction hash returned successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Input or output mint not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during swap operation"
//	@Failure		503				{object}	apperrors.ErrorPublic	"External service unavailable"
//	@Router			/wallet/swap [post]
func (h *WalletHandler) SwapCurrencies(c fiber.Ctx) error {
	var req model.SwapReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	txHash, err := h.WalletService.Swap(c.Context(), claims.UserID, req.InputMint, req.OutputMint, req.Amount)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"tx_hash": txHash})
}

// GetUserWalletTokens godoc
//
//	@Summary		Get user's wallet tokens
//	@Description	Retrieves a list of tokens associated with the authenticated user's wallet, supporting pagination, ordering, and filtering via query parameters.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization				header		string					true	"Authorization Bearer token"
//	@Param			opts.pagination.page_size	query		uint64					false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64					false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string					false	"Field to order by"
//	@Param			opts.order.order_type		query		string					false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string					false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"Filter operator"
//	@Param			opts.filters[0].value		query		string					false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"Filter OR condition"
//	@Success		200							{array}		model.WalletToken		"List of user's wallet tokens returned successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/wallet/token [get]
func (h *WalletHandler) GetUserWalletTokens(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	tokens, err := h.WalletService.FindUserWalletTokens(c.Context(), &req.Opts, claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(tokens)
}

// EditUserWalletToken godoc
//
//	@Summary		Edit a token in the user's wallet
//	@Description	Updates the properties of a token associated with the authenticated user's wallet.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.WalletTokenEditReq	true	"Token edit request payload"
//	@Success		200				{object}	nil							"Token updated successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid credentials"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error"
//	@Router			/wallet/token [put]
func (h *WalletHandler) EditUserWalletToken(c fiber.Ctx) error {
	var token model.WalletTokenEditReq
	if err := c.Bind().JSON(&token); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.WalletService.EditUserWalletToken(c.Context(), claims.UserID, &token)
	if err != nil {
		return err
	}

	return nil
}

// AddTokenToUserWallet godoc
//
//	@Summary		Add a token to the user's wallet
//	@Description	Adds a new token to the authenticated user's wallet. The token mint address must be valid and not already present in the user's wallet.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.WalletToken		true	"Token to add to the user's wallet"
//	@Success		200				{object}	nil						"Token added successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or failed to decode mint public key"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		409				{object}	apperrors.ErrorPublic	"Token already added to user's wallet"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/wallet/token [post]
func (h *WalletHandler) AddTokenToUserWallet(c fiber.Ctx) error {
	var token model.WalletToken
	if err := c.Bind().JSON(&token); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}
	token.UserID = claims.UserID

	err := h.WalletService.AddTokenToUserWallet(c.Context(), &token)
	if err != nil {
		return err
	}

	return nil
}

// RemoveTokenFromUserWallet godoc
//
//	@Summary		Remove a token from the user's wallet
//	@Description	Removes a token from the authenticated user's wallet by its mint address. The mint address must be provided as a path parameter.
//	@Tags			wallet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			mint			path		string					true	"Token mint address to remove from the user's wallet"
//	@Success		200				{object}	nil						"Token removed successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or missing mint address"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/wallet/token/{mint} [delete]
func (h *WalletHandler) RemoveTokenFromUserWallet(c fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.WalletService.RemoveTokenFromUserWallet(c.Context(), claims.UserID, mint)
	if err != nil {
		return err
	}

	return nil
}

// @Summary		Get all swap notifications for the authenticated user
// @Description	Retrieves all swap notifications associated with the authenticated user's wallet.
// @Tags			wallet
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			Authorization	header		string					true	"Authorization Bearer token"
// @Success		200				{array}		model.TxNotification	"List of transaction notifications"
// @Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
// @Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
// @Router			/wallet/tx-notifications [get]
func (h *WalletHandler) GetAllTxNotifications(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	notifications, err := h.WalletService.GetAllTxNotifications(c.Context(), claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(notifications)
}

// @Summary		Delete transaction notifications
// @Description	Deletes one or more transaction notifications for the authenticated user by their notification IDs.
// @Tags			wallet
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			Authorization	header		string							true	"Authorization Bearer token"
// @Param			request			body		model.TxNotificationDeleteReq	true	"List of notification IDs to delete"
// @Success		200				{object}	nil								"Notifications deleted successfully"
// @Failure		400				{object}	apperrors.ErrorPublic			"Invalid request data or missing notification IDs"
// @Failure		401				{object}	apperrors.ErrorPublic			"Authentication required or invalid credentials"
// @Failure		500				{object}	apperrors.ErrorPublic			"Internal server error"
// @Router			/wallet/tx-notifications [delete]
func (h *WalletHandler) DeleteTxNotifications(c fiber.Ctx) error {
	var req model.TxNotificationDeleteReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.WalletService.DeleteTxNotifications(c.Context(), claims.UserID, req.NotificationIDs)
	if err != nil {
		return err
	}

	return nil
}
