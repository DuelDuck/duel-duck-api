package v1

import (
	"github.com/gofiber/fiber/v3"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
)

type FAQHandler struct {
	FAQService  *service.FAQService
	AuthService *service.AuthService
}

func NewFAQHandler(
	faqService *service.FAQService,
	authService *service.AuthService,
) *FAQHandler {
	return &FAQHandler{
		FAQService:  faqService,
		AuthService: authService,
	}
}

func (h *FAQHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	faq := app.Group("/faq")
	faq.Use(auth.FAQAuthMiddleware)

	{
		faq.Get("/", h.GetAllFAQ)
		faq.Post("/", h.CreateFAQ)
		faq.Put("/mark", h.AddFAQMark)
	}
}

// GetAllFAQ godoc
//
//	@Summary		Get all FAQs
//	@Description	Retrieve a list of all FAQs with optional filtering, sorting, and pagination.
//	@Tags			faq
//	@Accept			json
//	@Produce		json
//
//	@Param			answered					query		boolean												false	"Filter by answered status"
//	@Param			me							query		boolean												false	"Filter only current user's questions"
//	@Param			share_id					query		uint												false	"Filter by specific shared FAQ ID"
//	@Param			opts.pagination.page_size	query		uint64												false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64												false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string												false	"Field to order by"				Enums(time,marks_value)	"Order by (time or marks_value)"
//	@Param			opts.order.order_type		query		string												false	"Order type"					Enums(desc,asc)			"Order type (asc or desc)"
//	@Param			opts.filters[0].column		query		string												false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string												false	"Filter operator"
//	@Param			opts.filters[0].value		query		string												false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool												false	"Filter OR condition"
//
//	@Success		200							{object}	object{page_size=int,page_num=int,faqs=[]model.FAQ}	"List of FAQs with pagination info"
//	@Failure		400							{object}	apperrors.ErrorPublic								"Invalid request parameters"
//	@Failure		401							{object}	apperrors.ErrorPublic								"Unauthorized - Invalid or missing claims"
//	@Failure		500							{object}	apperrors.ErrorPublic								"Internal server error"
//
//	@Router			/faq [get]
func (h *FAQHandler) GetAllFAQ(c fiber.Ctx) error {
	user, err := getFAQUserFromContext(c)
	if err != nil {
		return err
	}

	var faqParams model.FAQListQuery
	if err := c.Bind().Query(&faqParams); err != nil {
		return apperrors.BadRequest("failed to parse request", err)
	}

	faqs, err := h.FAQService.GetAllFAQ(c.Context(), user, &faqParams)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"page_size": faqParams.Opts.Pagination.PageSize,
		"page_num":  faqParams.Opts.Pagination.PageNum,
		"faqs":      faqs,
	})
}

// CreateFAQ godoc
//
//	@Summary		Create a new FAQ
//	@Description	Creates a new FAQ entry with the provided question and optional images.
//	@Tags			faq
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			question	formData	string					true	"Question text"
//	@Param			images		formData	[]file					false	"Images to upload"
//	@Success		200			{object}	model.FAQ				"FAQ created successfully"
//	@Failure		400			{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401			{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		500			{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/faq [post]
func (h *FAQHandler) CreateFAQ(c fiber.Ctx) error {
	user, err := getFAQUserFromContext(c)
	if err != nil {
		return err
	}

	var req model.CreateFAQ

	req.Question = c.FormValue("question")

	form, err := c.MultipartForm()
	if err != nil {
		return err
	}

	files, ok := form.File["images"]
	if ok {
		req.Images = files
	}

	if err := req.Validate(); err != nil {
		return err
	}

	faqQuestion, err := h.FAQService.AddFAQQuestion(c.Context(), user, &req)
	if err != nil {
		return err
	}

	return c.JSON(faqQuestion)
}

// AddFAQMark godoc
//
//	@Summary		Add a mark to an FAQ
//	@Description	Adds a mark to an existing FAQ entry.
//	@Tags			faq
//	@Accept			json
//	@Produce		json
//	@Param			request	body		model.AddFAQMarkReq		true	"Mark request data"
//	@Success		200		{object}	object{success=bool}	"Success status"
//	@Failure		400		{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401		{object}	apperrors.ErrorPublic	"Unauthorized - Invalid or missing claims"
//	@Failure		500		{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/faq/mark [put]
func (h *FAQHandler) AddFAQMark(c fiber.Ctx) error {
	user, err := getFAQUserFromContext(c)
	if err != nil {
		return err
	}

	var req model.AddFAQMarkReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	if err := req.Validate(); err != nil {
		return err
	}

	err = h.FAQService.AddFAQMark(
		c.Context(),
		user,
		&model.FAQMark{
			QuestionID: req.QuestionID,
			State:      req.State,
		})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"success": true})
}

func getFAQUserFromContext(c fiber.Ctx) (*model.FAQUser, error) {
	user, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		user, ok := c.Locals("anonymous_claims").(model.FAQUser)
		if !ok {
			return nil, apperrors.Unauthorized("claims not found")
		}
		return &model.FAQUser{
			ID:        user.ID,
			Anonymous: true,
		}, nil
	}

	return &model.FAQUser{
		ID:        user.UserID,
		Anonymous: false,
	}, nil
}
