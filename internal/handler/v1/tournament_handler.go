package v1

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/internal/telegram"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
)

type TournamentHandler struct {
	TournamentService *service.TournamentService
	TournamentTGBot   *telegram.TournamentTGBot
}

func NewTournament(
	tournamentService *service.TournamentService,
	tournamentTGBot *telegram.TournamentTGBot,
) *TournamentHandler {
	return &TournamentHandler{
		TournamentService: tournamentService,
		TournamentTGBot:   tournamentTGBot,
	}
}
func (h *TournamentHandler) RegisterRoutes(
	app *fiber.App,
	auth *AuthHandler,
	admin *AdminHandler,
) {
	// Endpoints allowed for Unauthorized clients
	tournament := app.Group("/tournament")
	{
		tournament.Get("/", h.Tournaments)
		tournament.Get("/rewards", h.Rewards)
		tournament.Get("/leaderboard", h.Leaderboard)
		tournament.Post("/offer", h.OfferTournament)
	}

	// Authorized only
	tournament.Use(auth.AuthMiddleware)
	{
		tournament.Get("/rank", h.TournamentRank)
		tournament.Get("/my", h.UserTournaments)
	}

	// Admins only
	tournament.Use(admin.CheckForAdminPermissions)
	{
		tournament.Post("/", h.CreateTournament)
		tournament.Put("/rewards", h.UpdateRewards)
	}
}

// Leaderboard godoc
//
//	@Summary		Get tournament leaderboard
//	@Description	Retrieves the leaderboard for a specific tournament with pagination, filtering, and ordering options. Returns a list of tournament participants ranked by their performance.
//	@Tags			tournament
//	@Accept			json
//	@Produce		json
//	@Param			tournament_id				query		string															true	"Tournament ID (UUID format)"
//	@Param			opts.pagination.page_size	query		uint64															false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64															false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string															false	"Field to order by"				default("pnl")
//	@Param			opts.order.order_type		query		string															false	"Order type"					Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string															false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string															false	"Filter operator"
//	@Param			opts.filters[0].value		query		string															false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool															false	"Filter OR condition"
//	@Success		200							{object}	object{leaders=[]model.TournamentLeaderboard,leader_count=int}	"Tournament leaderboard retrieved successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic											"Invalid request data or validation errors"
//	@Failure		500							{object}	apperrors.ErrorPublic											"Internal server error"
//	@Router			/tournament/leaderboard [get]
func (h *TournamentHandler) Leaderboard(c fiber.Ctx) error {
	var req model.TournamentLeaderboardReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	leaders, leaderCount, err := h.TournamentService.Leaderboard(c.Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"leaders":      leaders,
		"leader_count": leaderCount,
	})
}

// Tournaments godoc
//
//	@Summary		List all tournaments
//	@Description	Retrieves a paginated list of all tournaments with optional filtering, ordering, and pagination parameters.
//	@Tags			tournament
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
//	@Success		200							{array}		model.Tournament		"List of tournaments returned successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/tournament [get]
func (h *TournamentHandler) Tournaments(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	tournaments, err := h.TournamentService.GetAll(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(tournaments)
}

// TournamentRank godoc
//
//	@Summary		Get user's rank in a tournament leaderboard
//	@Description	Retrieves the authenticated user's rank and stats in the specified tournament leaderboard. Supports filtering, ordering, and pagination via query parameters.
//	@Tags			tournament
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization				header		string					true	"Authorization Bearer token"
//	@Param			tournament_id				query		string					true	"Tournament ID (UUID)"
//	@Param			opts.order.order_by			query		string					false	"Field to order by"
//	@Param			opts.order.order_type		query		string					false	"Order type"	Enums(desc,asc)	default("")	"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string					false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string					false	"Filter operator"
//	@Param			opts.filters[0].value		query		string					false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool					false	"Filter OR condition"
//	@Success		200							{object}	model.TournamentRank	"User's tournament rank and stats returned successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid credentials"
//	@Failure		404							{object}	apperrors.ErrorPublic	"User not found in tournament leaderboard"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/tournament/rank [get]
func (h *TournamentHandler) TournamentRank(c fiber.Ctx) error {
	var req model.TournamentLeaderboardReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	leader, err := h.TournamentService.TournamentRank(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(leader)
}

// UserTournaments godoc
//
//	@Summary		Get user's tournaments
//	@Description	Retrieves tournaments that the authenticated user participates in. Supports filtering, ordering, and pagination via query parameters.
//	@Tags			tournament
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
//	@Success		200							{array}		model.TournamentUserShow	"User's tournaments retrieved successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic		"Invalid request params"
//	@Failure		401							{object}	apperrors.ErrorPublic		"Authentication required or invalid credentials"
//	@Failure		500							{object}	apperrors.ErrorPublic		"Internal server error"
//	@Router			/tournament/my [get]
func (h *TournamentHandler) UserTournaments(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	tournaments, err := h.TournamentService.UserTournaments(c.Context(), &req.Opts, claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(tournaments)
}

// OfferTournament godoc
//
//	@Summary		Create tournament offer
//	@Description	Creates a new tournament offer and sends notification via Telegram bot
//	@Tags			tournament
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.TournamentOfferReq	true	"Tournament offer request data"
//	@Success		200		{object}	nil							"Tournament offer created successfully"
//	@Failure		400		{object}	apperrors.ErrorPublic		"Invalid request data"
//	@Failure		500		{object}	apperrors.ErrorPublic		"Internal server error during offer creation or notification"
//	@Router			/tournament/offer [post]
func (h *TournamentHandler) OfferTournament(c fiber.Ctx) error {
	var req model.TournamentOfferReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	offer, err := h.TournamentService.CreateOffer(c.Context(), &req)
	if err != nil {
		return err
	}

	return h.TournamentTGBot.Notify(offer)
}

// CreateTournament godoc
//
//	@Summary		Create new tournament
//	@Description	Creates a new tournament with specified rewards and tournament data. Requires admin permissions.
//	@Tags			tournament
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.TournamentCreateReq	true	"Tournament creation request data"
//	@Success		200				{object}	nil							"Tournament created successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data or invalid rewards parameters"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid credentials"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during tournament creation"
//	@Router			/tournament [post]
func (h *TournamentHandler) CreateTournament(c fiber.Ctx) error {
	var req model.TournamentCreateReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	if err := req.Rewards.Valid(); err != nil {
		return apperrors.BadRequest("invalid rewards params", err)
	}

	err := h.TournamentService.Create(c.Context(), &req.Tournament, req.Rewards, claims.UserID)
	if err != nil {
		return err
	}

	return nil
}

// Rewards godoc
//
//	@Summary		Get tournament rewards
//	@Description	Retrieves rewards information for a specific tournament by tournament ID
//	@Tags			tournament
//	@Accept			json
//	@Produce		json
//	@Param			tournament_id	query		string					true	"Tournament ID (UUID format)"
//	@Success		200				{object}	model.TournamentRewards	"Tournament rewards retrieved successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request params or missing tournament ID"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Tournament not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/tournament/rewards [get]
func (h *TournamentHandler) Rewards(c fiber.Ctx) error {
	var req model.TournamentRewardsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	if req.TournamentID == uuid.Nil {
		return apperrors.BadRequest("invalid request params")
	}

	rewards, err := h.TournamentService.Rewards(c.Context(), req.TournamentID)
	if err != nil {
		return err
	}

	return c.JSON(rewards)
}

// UpdateRewards godoc
//
//	@Summary		Update tournament rewards
//	@Description	Updates the rewards configuration for a specific tournament. Only authorized users with appropriate permissions can modify tournament rewards.
//	@Tags			tournament
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string								true	"Authorization Bearer token"
//	@Param			request			body		model.TournamentRewardsUpdateReq	true	"Tournament rewards update request"
//	@Success		200				{object}	nil									"Tournament rewards updated successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic				"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic				"Authentication required or invalid token"
//	@Router			/tournament/rewards [put]
func (h *TournamentHandler) UpdateRewards(c fiber.Ctx) error {
	var req model.TournamentRewardsUpdateReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	if err := req.Rewards.Valid(); err != nil {
		return apperrors.BadRequest("invalid rewards params", err)
	}

	err := h.TournamentService.UpdateRewards(c.Context(), req.Rewards, req.TournamentID, claims.UserID)
	if err != nil {
		return err
	}

	return nil
}
