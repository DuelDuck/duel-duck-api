package v1

import (
	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
)

type StatsHandler struct {
	UserService *service.UserService
}

func NewStatsHandler(userService *service.UserService) *StatsHandler {
	return &StatsHandler{
		UserService: userService,
	}
}
func (h *StatsHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	stats := app.Group("/stats")

	// Unauthorized only
	user := stats.Group("/user")
	{
		user.Get("/count", h.GetUsersCount)
	}

	// Authorized only
	user.Use(auth.AuthMiddleware)
	{
		user.Get("/", h.UserReferralStats)
		user.Get("/referrals", h.UserReferrals)
	}
}

// UserReferralStats godoc
//
//	@Summary		Get referral statistics for the authenticated user
//	@Description	Retrieves the referral statistics (such as total referrals, income, etc.) for the currently authenticated user.
//	@Tags			stats
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Success		200				{object}	model.UserReferralStats	"Referral statistics retrieved successfully"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during referral stats retrieval"
//	@Router			/stats/user [get]
func (h *StatsHandler) UserReferralStats(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	referralStats, err := h.UserService.ReferralStatsByID(c.Context(), claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(referralStats)
}

// UserReferrals godoc
//
//	@Summary		Get referral list for the authenticated user
//	@Description	Retrieves the list of users referred by the currently authenticated user, with optional filtering, ordering, and pagination via query parameters.
//	@Tags			stats
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
//	@Success		200							{array}		model.UserReferralShow	"List of referred users retrieved successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request data or query structure"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error during referral retrieval"
//	@Router			/stats/user/referrals [get]
func (h *StatsHandler) UserReferrals(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	referralStats, err := h.UserService.UserReferralsByID(c.Context(), claims.UserID, &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(referralStats)
}

// GetUsersCount godoc
//
//	@Summary		Get total user count
//	@Description	Retrieves the total number of registered users in the system.
//	@Tags			stats
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{count=int}		"Total user count returned successfully"
//	@Failure		500	{object}	apperrors.ErrorPublic	"Internal server error during user count retrieval"
//	@Router			/stats/user/count [get]
func (h *StatsHandler) GetUsersCount(c fiber.Ctx) error {
	count, err := h.UserService.CountAllUsers(c.Context())
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"count": count,
	})
}
