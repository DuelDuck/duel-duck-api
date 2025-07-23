package v1

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
)

type AdminHandler struct {
	DuelService           *service.DuelService
	UserService           *service.UserService
	WalletService         *service.WalletService
	AdvertiserLinkService *service.AdvertiserLinkService
	FileService           *service.FileService
}

func NewAdminHandler(
	duelService *service.DuelService,
	userService *service.UserService,
	walletService *service.WalletService,
	advertiserLinkService *service.AdvertiserLinkService,
	fileService *service.FileService,
) *AdminHandler {
	return &AdminHandler{
		DuelService:           duelService,
		UserService:           userService,
		WalletService:         walletService,
		AdvertiserLinkService: advertiserLinkService,
		FileService:           fileService,
	}
}
func (h *AdminHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	adminGroup := app.Group("/admin")

	adminGroup.Use(auth.AuthMiddleware)
	adminGroup.Use(h.CheckForAdminPermissions)

	{
		adminGroup.Post("upload-image", h.UploadImage)
		adminGroup.Post("delete-image", h.DeleteImage)
	}

	duel := adminGroup.Group("/duel")
	{
		crypto := duel.Group("/crypto")
		{
			crypto.Post("/", h.CreateCryptoDuel)
			crypto.Put("/approve", h.ApproveCryptoDuel)
			crypto.Put("/resolve", h.ResolveCryptoDuel)
			crypto.Put("/cancel", h.CancelCryptoDuel)
		}

		// duel management
		duel.Post("/", h.CreateDuel)
		duel.Put("/", h.EditDuel)
		duel.Put("/assign-to-tournament", h.AssignDuelToTournament)

		duel.Put("/resolve", h.ResolveDuel)
		duel.Put("/approve", h.ApproveDuel)
		duel.Put("/cancel", h.CancelDuel)

		// entity management
		duel.Post("/entity", h.CreateDuelEntity)
		duel.Post("/subtopic", h.CreateDuelSubtopic)
		duel.Put("/entity", h.UpdateDuelEntity)
		duel.Put("/subtopic", h.UpdateDuelSubtopic)

		duel.Get("/moderator-stats", h.GetModeratorStats)
	}

	advertiserLink := adminGroup.Group("/advertiser-link")
	{
		advertiserLink.Post("/", h.CreateAdvertiserLink)
		advertiserLink.Get("/", h.GetAllAdvertiserLinks)
		advertiserLink.Put("/", h.EditAdvertiserLink)
		advertiserLink.Delete("/:id", h.DeleteAdvertiserLink)
	}
}

// CreateDuel godoc
//
//	@Summary		Create new d
//	@Summary		Create new duel
//	@Description	Create a new duel by admin with the provided duel data
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.CreateDuelReq		true	"Duel creation data"
//	@Success		200				{object}	object{}				"Duel created successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Unauthorized - claims not found or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden - admin permissions required"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel creation"
//	@Router			/admin/duel/ [post]
func (h *AdminHandler) CreateDuel(c fiber.Ctx) error {
	var req model.CreateDuelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.DuelService.CreateNewDuelAdmin(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return nil
}

//	@Summary		Create ne
//
// CreateCryptoDuel godoc
//
//	@Summary		Create new crypto duel
//	@Description	Create a new crypto duel by admin with the provided duel data
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.CreateDuelReq		true	"Crypto duel creation data"
//	@Success		200				{object}	model.Duel				"Crypto duel created successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Unauthorized - claims not found or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden - admin permissions required"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during crypto duel creation"
//	@Router			/admin/duel/crypto [post]
func (h *AdminHandler) CreateCryptoDuel(c fiber.Ctx) error {
	var req model.CreateDuelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	duel, err := h.DuelService.CreateNewCryptoDuelAdmin(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(duel)
}

func (h *AdminHandler) CheckForAdminPermissions(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	user, err := h.UserService.GetByID(c.Context(), claims.UserID)
	if err != nil {
		return apperrors.Unauthorized("failed to fetch user")
	}

	if !user.Role.Admin() {
		return apperrors.Unauthorized("permission denied")
	}

	return c.Next()
}

// ResolveDuel godoc
//
//	@Summary		Resolve duck point duel by admin
//	@Description	Resolve a duck point duel by admin with the provided resolution data
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.DuelResolveReq	true	"Duel resolution data"
//	@Success		200				{object}	object{}				"Duel resolved successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Unauthorized - claims not found or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden - admin permissions required"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel resolution"
//	@Router			/admin/duel/resolve [put]
func (h *AdminHandler) ResolveDuel(c fiber.Ctx) error {
	var req model.DuelResolveReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.DuelService.ResolveDuckPointDuelByAdmin(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return nil
}

// ResolveCryptoDuel godoc
//
//	@Summary		Resolve crypto duel by admin
//	@Description	Resolve a crypto duel by admin with the provided resolution data and return transaction hashes
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.DuelResolveReq		true	"Duel resolution data"
//	@Success		200				{object}	object{tx_hashes=[]string}	"Crypto duel resolved successfully with transaction hashes"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Unauthorized - claims not found or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden - admin permissions required"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during crypto duel resolution"
//	@Router			/admin/duel/crypto/resolve [put]
func (h *AdminHandler) ResolveCryptoDuel(c fiber.Ctx) error {
	var req model.DuelResolveReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	txHashes, err := h.DuelService.ResolveCryptoDuelByAdmin(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"tx_hashes": txHashes})
}

// ApproveDuel godoc
//
//	@Summary		Approve duel by admin
//	@Description	Approve a duel by admin with the provided approval data and make it public after creation
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.DuelApproveReq	true	"Duel approval data"
//	@Success		200				{object}	object{}				"Duel approved successfully and made public"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Unauthorized - claims not found or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden - admin permissions required"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel approval"
//	@Router			/admin/duel/approve [put]
func (h *AdminHandler) ApproveDuel(c fiber.Ctx) error {
	var req model.DuelApproveReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.DuelService.ApproveDuel(c.Context(), claims.UserID, req)
	if err != nil {
		return err
	}

	return nil
}

// ApproveCryptoDuel godoc
//
//	@Summary		Approve crypto duel by admin
//	@Description	Approve a crypto duel by admin with the provided approval data and make it public after creation
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.DuelApproveReq		true	"Crypto duel approval data"
//	@Success		200				{object}	model.JoinCryptoDuelResp	"Crypto duel approved successfully and made public"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Unauthorized - claims not found or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden - admin permissions required"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during crypto duel approval"
//	@Router			/admin/duel/crypto/approve [put]
func (h *AdminHandler) ApproveCryptoDuel(c fiber.Ctx) error {
	var req model.DuelApproveReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	resp, err := h.DuelService.ApproveCryptoDuel(c.Context(), claims.UserID, req)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

// CancelDuel godoc
//
//	@Summary		Cancel a Duck Point duel by admin
//	@Description	Cancel an existing Duck Point duel as an admin. Requires admin authorization. The duel is identified and canceled based on the provided request data.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.DuelCancelReq		true	"Duel cancellation data"
//	@Success		200				{object}	object{}				"Duel canceled successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel cancellation"
//	@Router			/admin/duel/cancel [put]
func (h *AdminHandler) CancelDuel(c fiber.Ctx) error {
	var req model.DuelCancelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.DuelService.CancelDuckPointDuelByAdmin(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return nil
}

// CancelCryptoDuel godoc
//
//	@Summary		Cancel a crypto duel by admin
//	@Description	Cancel an existing crypto duel as an admin. Requires admin authorization. The duel is identified and canceled based on the provided request data. Returns the transaction hashes of the cancellation operations.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.DuelCancelReq			true	"Duel cancellation data"
//	@Success		200				{object}	object{tx_hashes=[]string}	"Duel canceled successfully, returning transaction hashes"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden or insufficient permissions"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during duel cancellation"
//	@Failure		503				{object}	apperrors.ErrorPublic		"External services are unavailable"
//	@Router			/admin/duel/crypto/cancel [put]
func (h *AdminHandler) CancelCryptoDuel(c fiber.Ctx) error {
	var req model.DuelCancelReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	txHashes, err := h.DuelService.CancelCryptoDuelByAdmin(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"tx_hashes": txHashes})
}

// EditDuel godoc
//
//	@Summary		Edit an existing duel as admin
//	@Description	Allows an admin to edit the details of an existing duel. Requires admin authorization. The duel to be edited and the new details are provided in the request body.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.DuelAdminEditReq	true	"Duel edit data"
//	@Success		200				{object}	nil						"Duel edited successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Duel not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel edit"
//	@Router			/admin/duel [put]
func (h *AdminHandler) EditDuel(c fiber.Ctx) error {
	var req model.DuelAdminEditReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := h.DuelService.EditDuel(c.Context(), &req)
	if err != nil {
		return err
	}

	return nil
}

// AssignDuelToTournament godoc
//
//	@Summary		Assign a duel to a tournament as admin
//	@Description	Allows an admin to assign or unassign a duel to a tournament. Requires admin authorization. The duel and tournament IDs are provided in the request body.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string									true	"Authorization Bearer token"
//	@Param			request			body		model.DuelAdminAssignToTournamentReq	true	"Duel and tournament assignment data"
//	@Success		200				{object}	nil										"Duel assigned to tournament successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic					"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic					"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic					"Access forbidden or insufficient permissions"
//	@Failure		500				{object}	apperrors.ErrorPublic					"Internal server error during assignment"
//	@Router			/admin/duel/assign-to-tournament [put]
func (h *AdminHandler) AssignDuelToTournament(c fiber.Ctx) error {
	var req model.DuelAdminAssignToTournamentReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := h.DuelService.SetTournamentID(c.Context(), req.DuelID, req.TournamentID)
	if err != nil {
		return err
	}

	return nil
}

// CreateDuelEntity godoc
//
//	@Summary		Create a new duel entity
//	@Description	Allows an admin to create a new duel entity by providing entity details. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.CreateDuelEntityReq	true	"Duel entity creation data"
//	@Success		200				{object}	model.DuelEntity			"Duel entity created successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden or insufficient permissions"
//	@Failure		409				{object}	apperrors.ErrorPublic		"Duel entity already exists"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during duel entity creation"
//	@Router			/admin/duel/entity [post]
func (h *AdminHandler) CreateDuelEntity(c fiber.Ctx) error {
	var req model.CreateDuelEntityReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	entity, err := h.DuelService.CreateDuelEntity(c.Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(entity)
}

// UpdateDuelEntity godoc
//
//	@Summary		Update an existing duel entity
//	@Description	Allows an admin to update the details of an existing duel entity. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.DuelEntity		true	"Duel entity update data"
//	@Success		200				{object}	nil						"Duel entity updated successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Duel entity not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel entity update"
//	@Router			/admin/duel/entity [put]
func (h *AdminHandler) UpdateDuelEntity(c fiber.Ctx) error {
	entity := new(model.DuelEntity)
	if err := c.Bind().JSON(entity); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := h.DuelService.UpdateDuelEntity(c.Context(), entity)
	if err != nil {
		return err
	}

	return nil
}

// CreateDuelSubtopic godoc
//
//	@Summary		Create a new duel subtopic
//	@Description	Allows an admin to create a new duel subtopic with the provided data. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.CreateDuelSubtopicReq	true	"Duel subtopic creation data"
//	@Success		200				{object}	model.DuelSubtopic			"Duel subtopic created successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden or insufficient permissions"
//	@Failure		409				{object}	apperrors.ErrorPublic		"Duel subtopic already exists"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during duel subtopic creation"
//	@Router			/admin/duel/subtopic [post]
func (h *AdminHandler) CreateDuelSubtopic(c fiber.Ctx) error {
	var req model.CreateDuelSubtopicReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	subtopic, err := h.DuelService.CreateDuelSubtopic(c.Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(subtopic)
}

// UpdateDuelSubtopic godoc
//
//	@Summary		Update an existing duel subtopic
//	@Description	Allows an admin to update an existing duel subtopic with the provided data. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.DuelSubtopic		true	"Duel subtopic update data"
//	@Success		200				{object}	nil						"Duel subtopic updated successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Duel subtopic not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during duel subtopic update"
//	@Router			/admin/duel/subtopic [put]
func (h *AdminHandler) UpdateDuelSubtopic(c fiber.Ctx) error {
	subtopic := new(model.DuelSubtopic)
	if err := c.Bind().JSON(subtopic); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := h.DuelService.UpdateDuelSubtopic(c.Context(), subtopic)
	if err != nil {
		return err
	}

	return nil
}

// UploadImage godoc
//
//	@Summary		Upload an image for admin use
//	@Description	Allows an admin to upload an image file. The uploaded image will be saved and a URL to the image will be returned.
//	@Tags			admin
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			image			formData	file						true	"Image file to upload"
//	@Success		200				{object}	object{image_url=string}	"Image uploaded successfully, returns image_url"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data or missing image file"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden or insufficient permissions"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during image upload"
//	@Router			/admin/upload-image [post]
func (h *AdminHandler) UploadImage(c fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	fileName, err := h.FileService.SaveAdminsFile(file)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"image_url": fileName,
	})
}

type DeleteFileReq struct {
	ImageURL string `json:"image_url"`
}

// DeleteImage godoc
//
//	@Summary		Delete an admin-uploaded image
//	@Description	Allows an admin to delete an image file previously uploaded. The image will be removed from storage.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		DeleteFileReq			true	"Image deletion request payload"
//	@Success		200				{object}	nil						"Image deleted successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during image deletion"
//	@Router			/admin/delete-image [post]
func (h *AdminHandler) DeleteImage(c fiber.Ctx) error {
	var req DeleteFileReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := h.FileService.RemoveFileAdmin(req.ImageURL)
	if err != nil {
		return err
	}

	return nil
}

// GetModeratorStats godoc
//
//	@Summary		Get moderator statistics
//	@Description	Retrieve statistics relevant to the moderator, such as counts and activity, with optional filtering, ordering, and pagination via query parameters.
//	@Tags			admin
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
//	@Success		200							{object}	model.ModeratorStats	"Moderator statistics retrieved successfully"
//	@Failure		400							{object}	apperrors.ErrorPublic	"Invalid request params or query structure"
//	@Failure		401							{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403							{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		500							{object}	apperrors.ErrorPublic	"Internal server error during moderator stats retrieval"
//	@Router			/admin/duel/moderator-stats [get]
func (h *AdminHandler) GetModeratorStats(c fiber.Ctx) error {
	var req model.OptsReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	stats, err := h.DuelService.GetModeratorStats(c.Context(), &req.Opts, claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(stats)
}

// CreateAdvertiserLink godoc
//
//	@Summary		Create a new advertiser link
//	@Description	Allows an admin to create a new advertiser link with the given data. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string							true	"Authorization Bearer token"
//	@Param			request			body		model.CreateAdvertiserLinkReq	true	"Advertiser link creation data"
//	@Success		200				{object}	model.AdvertiserLink			"Created advertiser link object"
//	@Failure		400				{object}	apperrors.ErrorPublic			"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic			"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic			"Access forbidden or insufficient permissions"
//	@Failure		500				{object}	apperrors.ErrorPublic			"Internal server error during advertiser link creation"
//	@Router			/admin/advertiser-link [post]
func (h *AdminHandler) CreateAdvertiserLink(c fiber.Ctx) error {
	var req model.CreateAdvertiserLinkReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := req.Validate()
	if err != nil {
		return err
	}

	link, err := h.AdvertiserLinkService.CreateAdvertiserLink(c.Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(link)
}

// GetAllAdvertiserLinks godoc
//
//	@Summary		Get all advertiser links with filtering and pagination
//	@Description	Allows an admin to retrieve all advertiser links. Supports filtering, sorting and pagination via query parameters. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization				header		string						true	"Authorization Bearer token"
//	@Param			opts.pagination.page_size	query		uint64						false	"Number of items per page"		default(10)
//	@Param			opts.pagination.page_num	query		uint64						false	"Page number (starting from 1)"	default(1)
//	@Param			opts.order.order_by			query		string						false	"Field to order by"
//	@Param			opts.order.order_type		query		string						false	"Order type"	Enums(desc,asc)	default("")
//	@Param			opts.filters[0].column		query		string						false	"Filter column name"
//	@Param			opts.filters[0].operator	query		string						false	"Filter operator"
//	@Param			opts.filters[0].value		query		string						false	"Filter value"
//	@Param			opts.filters[0].where_or	query		bool						false	"Filter OR condition"
//	@Success		200							{array}		model.AdvertiserLinkInfo	"List of advertiser links"
//	@Failure		400							{object}	apperrors.ErrorPublic		"Invalid request params or query structure"
//	@Failure		401							{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		403							{object}	apperrors.ErrorPublic		"Access forbidden or insufficient permissions"
//	@Failure		500							{object}	apperrors.ErrorPublic		"Internal server error during advertiser links retrieval"
//	@Router			/admin/advertiser-link [get]
func (h *AdminHandler) GetAllAdvertiserLinks(c fiber.Ctx) error {
	var req model.GetAllAdvertiserLinksReq
	if err := c.Bind().Query(&req); err != nil {
		return apperrors.BadRequest("failed to parse request", err)
	}

	links, err := h.AdvertiserLinkService.GetAllAdvertiserLinks(c.Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(links)
}

// EditAdvertiserLink godoc
//
//	@Summary		Edit an existing advertiser link
//	@Description	Allows an admin to update an existing advertiser link by ID. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			request			body		model.EditAdvertiserLinkReq	true	"Advertiser link edit data"
//	@Success		200				{object}	nil							"Advertiser link updated successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data or validation errors"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic		"Access forbidden or insufficient permissions"
//	@Failure		404				{object}	apperrors.ErrorPublic		"Advertiser link not found"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during advertiser link update"
//	@Router			/admin/advertiser-link [put]
func (h *AdminHandler) EditAdvertiserLink(c fiber.Ctx) error {
	var req model.EditAdvertiserLinkReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err := h.AdvertiserLinkService.EditAdvertiserLink(c.Context(), &req)
	if err != nil {
		return err
	}

	return nil
}

// DeleteAdvertiserLink godoc
//
//	@Summary		Delete an advertiser link by ID
//	@Description	Allows an admin to delete an advertiser link specified by link_id path parameter. Requires admin authorization.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			link_id			path		string					true	"UUID of the advertiser link to delete"
//	@Success		200				{object}	nil						"Advertiser link deleted successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request data"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		403				{object}	apperrors.ErrorPublic	"Access forbidden or insufficient permissions"
//	@Failure		404				{object}	apperrors.ErrorPublic	"Advertiser link not found"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during advertiser link deletion"
//	@Router			/admin/advertiser-link/{id} [delete]
func (h *AdminHandler) DeleteAdvertiserLink(c fiber.Ctx) error {
	linkIDStr := c.Params("id")
	if linkIDStr == "" {
		return apperrors.BadRequest("invalid request data")
	}

	linkID, err := uuid.Parse(linkIDStr)
	if err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	err = h.AdvertiserLinkService.DeleteAdvertiserLink(c.Context(), linkID)
	if err != nil {
		return err
	}

	return nil
}
