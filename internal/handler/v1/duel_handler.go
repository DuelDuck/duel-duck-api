package v1

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/handler/middleware"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
)

type DuelHandler struct {
	DuelService *service.DuelService
	CoinService *service.CoinService
}

func NewDuelHandler(
	duelService *service.DuelService,
	coinService *service.CoinService,
) (*DuelHandler, error) {
	authHandler := &DuelHandler{
		DuelService: duelService,
		CoinService: coinService,
	}

	return authHandler, nil
}

func (h *DuelHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	limiter := middleware.NewLimiter()

	cryptoDuelGroup := app.Group("/crypto-duel", auth.AuthMiddleware)
	{
		cryptoDuelGroup.Post("/", h.CreateCryptoDuel, limiter)
		cryptoDuelGroup.Post("/join", h.JoinCryptoDuel, limiter)

		// TODO: Move this endpoints to wallet logic handler
		cryptoDuelGroup.Get("/transaction-types", h.GetTransactionTypes)
		cryptoDuelGroup.Get("/transaction-history", h.GetTransactionHistory)
		cryptoDuelGroup.Get("/token-accounts", h.GetTokenAccountBalances)
	}

	duelApp := app.Group("/duel")
	{
		duelApp.Get("/all", h.GetAllDuels)
		duelApp.Get("/count", h.CountAllDuels)
		duelApp.Get("/counters", h.GetDuelsCountByTopic)
		duelApp.Get("/entities", h.FindAllEntities)
		duelApp.Get("/subtopics", h.FindAllSubtopics)

		duelApp.Get("/leaderboard", h.GetLeaderboard)
	}

	publicGroup := duelApp.Group("/public")
	{
		publicGroup.Get("/:id", h.GetDuelByIDUnauthorized)
	}

	duelApp.Use(auth.AuthMiddleware)
	{
		duelApp.Post("/", h.CreateDuel, limiter)
		duelApp.Get("/my", h.GetMyDuels)
		duelApp.Get("/all-with-joined", h.GetAllDuelsAuthorized)
		duelApp.Get("/counters-not-joined", h.GetIndividualDuelsCountByTopic)
		duelApp.Get("/participating", h.GetAllDuelsWhereParticipant)
		duelApp.Get("/history", h.GetMyHistoryDuels)
		duelApp.Post("/join", h.JoinDuel, limiter)
		duelApp.Get("/user-rank", h.GetUserRank)
		duelApp.Get("/:id", h.GetDuelByID)
	}
}

// CreateDuel godoc
//
//	@Summary		Create a new duel
//	@Description	Creates a new duel with the specified parameters. Requires authentication and proper request data.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		model.CreateDuelReq		true	"Duel creation request data"
//	@Success		200		{object}	model.Duel				"Duel created successfully"
//	@Failure		400		{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401		{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		500		{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/ [post]
func (h *DuelHandler) CreateDuel(c fiber.Ctx) error {
	var req model.CreateDuelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duel, err := h.DuelService.CreateNewDuel(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(duel)
}

// CreateCryptoDuel godoc
//
//	@Summary		Create a new crypto duel
//	@Description	Creates a new cryptocurrency-based duel with the specified parameters. Requires authentication and proper request data. This endpoint is specifically for creating duels that use cryptocurrency as payment.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.CreateDuelReq		true	"Duel creation request data"
//	@Success		200				{object}	model.Duel				"Duel created successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/crypto-duel [post]
func (h *DuelHandler) CreateCryptoDuel(c fiber.Ctx) error {
	var req model.CreateDuelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data", err)
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	resp, err := h.DuelService.CreateNewCryptoDuel(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

// JoinDuel godoc
//
//	@Summary		Join an existing duel
//	@Description	Allows an authenticated user to join an existing duel by providing the duel ID and their answer. The user must have sufficient balance to join the duel.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.JoinDuelReq		true	"Duel join request containing duel ID and player's answer"
//	@Success		200				{object}	model.Player			"Successfully joined the duel"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or insufficient balance"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Duel not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/join [post]
func (h *DuelHandler) JoinDuel(c fiber.Ctx) error {
	var req model.JoinDuelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	player, err := h.DuelService.JoinDuel(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(player)
}

// JoinCryptoDuel godoc
//
//	@Summary		Join a crypto duel
//	@Description	Allows an authenticated user to join an existing crypto duel by providing the duel ID and their answer. The user must have sufficient balance to join the duel.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		Bearer
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.JoinDuelReq			true	"Duel join request containing duel ID and player's answer"
//	@Success		200				{object}	model.JoinCryptoDuelResp	"Successfully joined the crypto duel"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data or insufficient balance"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Unauthorized - Invalid or missing claims"
//	@Failure		404				{object}	apperrors.ErrorPublic		"Crypto duel not found"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error"
//	@Router			/duel/join-crypto [post]
func (h *DuelHandler) JoinCryptoDuel(c fiber.Ctx) error {
	var req model.JoinDuelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	resp, err := h.DuelService.JoinCryptoDuel(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

// GetAllDuels godoc
//
//	@Summary		Get all duels
//	@Description	Get all duels with optional filtering, sorting and pagination
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			opts.pagination.page_size	query		uint64					false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64					false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string					false	"Field to order by"				default(created_at)
//	@Param			opts.order.order_type		query		string					false	"Order type"					Enums(desc,asc)	default("")	"Order type (asc or desc)"	Enums(asc,desc)	default(desc)
//	@Param			opts.filters[0].column		query		string					false	"First filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"First filter operator"
//	@Param			opts.filters[0].value		query		string					false	"First filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"First filter OR condition"
//	@Success		200							{array}		model.DuelShow			"List of duels"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Bad request"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/all [get]
func (h *DuelHandler) GetAllDuels(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	duels, err := h.DuelService.GetAllDuelsUnauthorized(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(duels)
}

// CountAllDuels godoc
//
//	@Summary		Count all duels
//	@Description	Get a count of all duels, with optional filtering.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			opts.filters[0].column		query		string					false	"First filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"First filter operator"
//	@Param			opts.filters[0].value		query		string					false	"First filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"First filter OR condition"
//	@Success		200							{integer}	int						"Total number of duels"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Bad request"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/count [get]
func (h *DuelHandler) CountAllDuels(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	count, err := h.DuelService.CountAllDuels(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(count)
}

// GetAllDuelsWhereParticipant godoc
//
//	@Summary		Get all duels where the user is a participant
//	@Description	Get all duels where the authenticated user is a participant, with optional filtering, sorting and pagination.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization				header		string					true	"Authorization Bearer token"
//	@Param			opts.pagination.page_size	query		uint64					false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64					false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string					false	"Field to order by"				default(created_at)
//	@Param			opts.order.order_type		query		string					false	"Order type"					Enums(desc,asc)	default("")	"Order type (asc or desc)"	Enums(asc,desc)	default(desc)
//	@Param			opts.filters[0].column		query		string					false	"First filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"First filter operator"
//	@Param			opts.filters[0].value		query		string					false	"First filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"First filter OR condition"
//	@Success		200							{array}		model.DuelShow			"List of duels"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Bad request"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/participating [get]
func (h *DuelHandler) GetAllDuelsWhereParticipant(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duels, err := h.DuelService.GetAllDuelsWhereParticipant(c.Context(), claims.UserID, &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(duels)
}

// GetAllDuelsAuthorized godoc
//
//	@Summary		Get all duels with user's participation status
//	@Description	Retrieves all duels and for each duel, indicates if the authenticated user is a participant. Supports optional filtering, sorting, and pagination.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization				header		string					true	"Authorization Bearer token"
//	@Param			opts.pagination.page_size	query		uint64					false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64					false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string					false	"Field to order by"
//	@Param			opts.order.order_type		query		string					false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string					false	"First filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"First filter operator"
//	@Param			opts.filters[0].value		query		string					false	"First filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"First filter OR condition"
//	@Success		200							{array}		model.DuelShow			"List of duels with participation status"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Bad request"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/all-with-joined [get]
func (h *DuelHandler) GetAllDuelsAuthorized(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duels, err := h.DuelService.GetAllDuels(c.Context(), claims.UserID, &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(duels)
}

// GetMyHistoryDuels godoc
//
//	@Summary		Get user's duel history
//	@Description	Retrieves the authenticated user's duel history with optional filtering, sorting, and pagination.
//	@Tags			duel
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
//	@Success		200							{array}		model.DuelShow			"List of user's historical duels"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request parameters"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/history [get]
func (h *DuelHandler) GetMyHistoryDuels(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duels, err := h.DuelService.GetMyHistoryDuels(c.Context(), claims.UserID, &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(duels)
}

// GetDuelsCountByTopic godoc
//
//	@Summary		Get duels count grouped by topic
//	@Description	Retrieves the count of duels grouped by topic, with optional filtering, sorting, and pagination. Does not require authentication.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			opts.filters[0].column		query		string					false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"Filter operator"
//	@Param			opts.filters[0].value		query		string					false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"Filter OR condition"
//	@Success		200							{object}	model.DuelTopicCounter	"Duels count grouped by topic"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request parameters"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/counters [get]
func (h *DuelHandler) GetDuelsCountByTopic(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	result, err := h.DuelService.GetDuelsCounterByTopic(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(result)
}

// GetIndividualDuelsCountByTopic godoc
//
//	@Summary		Get individual duels count grouped by topic (excluding joined duels)
//	@Description	Retrieves the count of duels grouped by topic for the authenticated user, excluding duels the user has already joined. Supports advanced filtering
//	@Tags			duel
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
//	@Success		200							{object}	model.DuelTopicCounter	"Duels count grouped by topic for the authenticated user"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request parameters"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/counters-not-joined [get]
func (h *DuelHandler) GetIndividualDuelsCountByTopic(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	result, err := h.DuelService.GetIndividualDuelsCounterByTopic(c.Context(), req.Opts, claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(result)
}

// GetMyDuels godoc
//
//	@Summary		Get my duels
//	@Description	Retrieves a paginated list of duels created by or assigned to the authenticated user. Supports advanced filtering, ordering, and pagination via query parameters.
//	@Tags			duel
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
//	@Success		200							{array}		model.DuelShow			"List of duels belonging to the authenticated user"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request parameters"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/my [get]
func (h *DuelHandler) GetMyDuels(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duel, err := h.DuelService.GetMyDuels(c.Context(), claims.UserID, &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(duel)
}

// GetDuelByIDUnauthorized godoc
//
//	@Summary		Get duel by ID (public)
//	@Description	Retrieve a duel by its ID without requiring authentication. Returns the duel details if found.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string					true	"Duel ID"
//	@Success		200	{object}	model.Duel				"Duel details"
//	@Failure		400	{object}	apperrors.ErrorPublic	"Invalid duel ID"
//	@Failure		404	{object}	apperrors.ErrorPublic	"Duel not found"
//	@Failure		500	{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/public/{id} [get]
func (h *DuelHandler) GetDuelByIDUnauthorized(c fiber.Ctx) error {
	duelID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperrors.BadRequest("invalid duel ID", err)
	}

	duel, err := h.DuelService.GetDuelByIDUnauthorized(c.Context(), duelID)
	if err != nil {
		return err
	}

	return c.JSON(duel)
}

// GetDuelByID godoc
//
//	@Summary		Get duel by ID (authorized)
//	@Description	Retrieve a duel by its ID for the authenticated user. Returns the duel details and the list of players. Requires authentication.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string													true	"Authorization Bearer token"
//	@Param			id				path		string													true	"Duel ID"
//	@Success		200				{object}	object{duel=model.DuelShow,players=[]model.PlayerShow}	"Duel details and players list"
//	@Failure		400				{object}	apperrors.ErrorPublic									"Invalid duel ID"
//	@Failure		401				{object}	apperrors.ErrorPublic									"Authentication required or invalid token"
//	@Failure		404				{object}	apperrors.ErrorPublic									"Duel not found"
//	@Failure		500				{object}	apperrors.ErrorPublic									"Internal server error"
//	@Router			/duel/{id} [get]
func (h *DuelHandler) GetDuelByID(c fiber.Ctx) error {
	duelID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperrors.BadRequest("invalid duel ID", err)
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duel, players, err := h.DuelService.GetDuelByID(c.Context(), duelID, claims.UserID, claims.Role)
	if err != nil {
		return err
	}

	return c.JSON(map[string]any{
		"duel":    duel,
		"players": players,
	})
}

// FindAllEntities godoc
//
//	@Summary		List all duel entities
//	@Description	Retrieve a list of all available duel entities with optional filtering, ordering, and pagination via query parameters.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			opts.pagination.page_size	query		uint64					false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64					false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string					false	"Field to order by"
//	@Param			opts.order.order_type		query		string					false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string					false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"Filter operator"
//	@Param			opts.filters[0].value		query		string					false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"Filter OR condition"
//	@Success		200							{array}		model.DuelEntity		"List of duel entities"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request params or query structure"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/entities [get]
func (h *DuelHandler) FindAllEntities(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	entities, err := h.DuelService.FindEntities(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(entities)
}

// FindAllSubtopics godoc
//
//	@Summary		List all duel subtopics
//	@Description	Retrieve a list of all available duel subtopics with optional filtering, ordering, and pagination via JSON body parameters.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			opts.pagination.page_size	query		uint64					false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64					false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string					false	"Field to order by"
//	@Param			opts.order.order_type		query		string					false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string					false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"Filter operator"
//	@Param			opts.filters[0].value		query		string					false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"Filter OR condition"
//	@Success		200							{array}		model.DuelSubtopic		"List of duel subtopics"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request params or query structure"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/duel/subtopics [get]
func (h *DuelHandler) FindAllSubtopics(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	entities, err := h.DuelService.FindSubtopics(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(entities)
}

// GetLeaderboard godoc
//
//	@Summary		Get leaderboard data
//	@Description	Retrieve leaderboard data with optional filtering, ordering, and pagination via query parameters
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Param			opts.pagination.page_size	query		uint64											false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64											false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string											false	"Field to order by"
//	@Param			opts.order.order_type		query		string											false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string											false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string											false	"Filter operator"
//	@Param			opts.filters[0].value		query		string											false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool											false	"Filter OR condition"
//	@Success		200							{object}	object{leaders=[]model.Leader,leader_count=int}	"Response with 'leaders' ([]model.Leader) and 'leader_count' (int)"
//	@Failure		400							{object}	apperrors.ErrorPublic							"Invalid request data or query structure"
//	@Failure		500							{object}	apperrors.ErrorPublic							"Internal server error during leaderboard retrieval"
//	@Router			/duel/leaderboard [get]
func (h *DuelHandler) GetLeaderboard(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	leaders, leaderCount, err := h.DuelService.GetLeaderboard(c.Context(), req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"leaders":      leaders,
		"leader_count": leaderCount,
	})
}

// GetUserRank godoc
//
//	@Summary		Get the authenticated user's leaderboard rank
//	@Description	Retrieves the leaderboard rank and stats for the authenticated user, with optional filtering, ordering, and pagination via query parameters. Returns only the current user's rank, not the full leaderboard.
//	@Tags			duel
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
//	@Success		200							{object}	model.Leader			"Authenticated user's leaderboard rank and stats"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request data or query structure"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		404							{object}	apperrors.ErrorPublic	"User not found in leaderboard"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error during leaderboard rank retrieval"
//	@Router			/duel/user-rank [get]
func (h *DuelHandler) GetUserRank(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	leaders, err := h.DuelService.GetUserLeaderboardRank(c.Context(), req.Opts, claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(leaders)
}

// GetTransactionTypes godoc
//
//	@Summary		Get transaction types by signatures
//	@Description	Retrieves the types of transactions for the provided list of transaction signatures.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			signatures		query		[]string				true	"List of transaction signatures to query types for"
//	@Success		200				{array}		model.TransactionType	"Transaction types retrieved successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or query parameters"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during transaction type retrieval"
//	@Router			/crypto-duel/transaction-types [get]
func (h *DuelHandler) GetTransactionTypes(c fiber.Ctx) error {
	var req model.GetTransactionTypesReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	resp, err := h.DuelService.GetTransactionsBySignatures(c.Context(), req.Signatures)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

// GetTransactionHistory godoc
//
//	@Summary		Get transaction history for the authenticated user's public address
//	@Description	Retrieves the transaction history for the authenticated user's public blockchain address. Requires authentication. Supports filtering and pagination via query parameters.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string							true	"Authorization Bearer token"
//	@Param			page_number		query		int								false	"Page number for pagination (starts from 1)"			default(1)
//	@Param			page_size		query		int								false	"Number of transactions per page"						default(20)
//	@Param			remove_spam		query		bool							false	"Whether to remove spam transactions from the results"	default	(true)
//	@Success		200				{object}	model.GetTransactionHistoryResp	"Transaction history retrieved successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic			"Invalid request data or query parameters"
//	@Failure		401				{object}	apperrors.ErrorPublic			"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic			"Internal server error"
//	@Failure		503				{object}	apperrors.ErrorPublic			"Explorer service unavailable"
//	@Router			/crypto-duel/transaction-history [get]
func (h *DuelHandler) GetTransactionHistory(c fiber.Ctx) error {
	var req model.GetTransactionHistoryReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	resp, err := h.DuelService.GetTransactionsByPublicAddress(c.Context(), claims.PublicAddress, &req)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

// GetTokenAccountBalances godoc
//
//	@Summary		Get token account balances for the authenticated user's public address
//	@Description	Retrieves the balances of all token accounts associated with the authenticated user's public blockchain address. Requires authentication. Supports filtering and pagination via query parameters.
//	@Tags			duel
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string							true	"Authorization Bearer token"
//	@Param			type			query		string							false	"Type of token account to filter (e.g. SPL, NFT, etc.)"	default("token")
//	@Param			page_number		query		int								false	"Page number for pagination (starts from 1)"			default(1)
//	@Param			page_size		query		int								false	"Number of token accounts per page"						default(20)
//	@Param			hide_zero		query		bool							false	"Hide token accounts with zero balance"
//	@Success		200				{object}	model.SolscanTokenAccountsResp	"Token account balances retrieved successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic			"Invalid request data or query parameters"
//	@Failure		401				{object}	apperrors.ErrorPublic			"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic			"Internal server error"
//	@Failure		503				{object}	apperrors.ErrorPublic			"External service unavailable"
//	@Router			/crypto-duel/token-accounts [get]
func (h *DuelHandler) GetTokenAccountBalances(c fiber.Ctx) error {
	var req model.GetTokenAccountsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	resp, err := h.DuelService.GetTokenAccounts(c.Context(), claims.PublicAddress, &req)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}
