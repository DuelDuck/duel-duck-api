package v1

import (
	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
)

type CoinHandler struct {
	CoinService *service.CoinService
}

func NewCoinHandler(coinService *service.CoinService) *CoinHandler {
	return &CoinHandler{CoinService: coinService}
}

func (h *CoinHandler) RegisterRoutes(app *fiber.App) {
	coinGroup := app.Group("/coins")

	{
		coinGroup.Post("/", h.GetAllCoins)
		coinGroup.Get("/solana-price", h.GetCurrentSolanaPrice)

		coinGroup.Get("/mint/:mint", h.GetTokenByMint)
		coinGroup.Get("/:name", h.GetTokenByName)
	}
}

// GetAllCoins godoc
//
//	@Summary		Get all coins with optional filtering, ordering, and pagination
//	@Description	Retrieve a list of all coins with support for filtering, ordering, and pagination via query parameters
//	@Tags			coins
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
//	@Success		200							{array}		model.Coin				"List of coins retrieved successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request params"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/coins [post]
func (h *CoinHandler) GetAllCoins(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	coins, err := h.CoinService.GetAllCoins(c.Context(), &req.Opts)
	if err != nil {
		return err
	}

	return c.JSON(coins)
}

// GetTokenByName godoc
//
//	@Summary		Get Solana token by name
//	@Description	Retrieve Solana token information by searching for tokens with the specified name
//	@Tags			coins
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string					true	"Token name to search for"
//	@Success		200		{array}		model.SolanaToken		"List of tokens matching the name"
//	@Failure		400		{object}	apperrors.ErrorPublic	"Invalid request data or empty token name"
//	@Failure		500		{object}	apperrors.ErrorPublic	"Internal server error during token retrieval"
//	@Router			/coins/{name} [get]
func (h *CoinHandler) GetTokenByName(c fiber.Ctx) error {
	token := c.Params("name")

	if token == "" {
		return apperrors.BadRequest("non-empty token query parameter expected")
	}

	tokens, err := h.CoinService.GetSolanaTokenByName(c.Context(), token)
	if err != nil {
		return err
	}

	return c.JSON(tokens)
}

// GetTokenByMint godoc
//
//	@Summary		Get Solana token by mint address
//	@Description	Retrieve Solana token information by searching for tokens with the specified mint address
//	@Tags			coins
//	@Accept			json
//	@Produce		json
//	@Param			mint	path		string					true	"Mint address to search for"
//	@Success		200		{object}	model.SolanaToken		"Token information retrieved successfully"
//	@Failure		400		{object}	apperrors.ErrorPublic	"Invalid request data or invalid mint address format"
//	@Failure		404		{object}	apperrors.ErrorPublic	"Token with provided mint address not found"
//	@Failure		500		{object}	apperrors.ErrorPublic	"Internal server error during token retrieval"
//	@Router			/coins/mint/{mint} [get]
func (h *CoinHandler) GetTokenByMint(c fiber.Ctx) error {
	mint := c.Params("mint")

	tokens, err := h.CoinService.GetSolanaTokenByMint(c.Context(), mint)
	if err != nil {
		return err
	}

	return c.JSON(tokens)
}

// GetCurrentSolanaPrice godoc
//
//	@Summary		Get current Solana price
//	@Description	Retrieve the current price of Solana cryptocurrency
//	@Tags			coins
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	object{solana_price=float64}	"Current Solana price retrieved successfully"
//	@Failure		500	{object}	apperrors.ErrorPublic			"Internal server error during price retrieval"
//	@Failure		503	{object}	apperrors.ErrorPublic			"External service unavailable"
//	@Router			/coins/solana-price [get]
func (h *CoinHandler) GetCurrentSolanaPrice(c fiber.Ctx) error {
	price, err := h.CoinService.GetCurrentSolanaPrice()
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"solana_price": price})
}
