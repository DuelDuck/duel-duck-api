package v1

import (
	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

type UserHandler struct {
	UserService *service.UserService
}

func NewUserHandler(
	userService *service.UserService,
) *UserHandler {
	return &UserHandler{
		UserService: userService,
	}
}

func (h *UserHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	userGroup := app.Group("/user")
	{
		userGroup.Get("/username/:username", h.FindUserByUsername)
	}

	userGroup.Use(auth.AuthMiddleware)
	{
		userGroup.Get("/", h.GetUser)
		userGroup.Put("/", h.EditUser)

		userGroup.Put("/profile-picture", h.SetProfilePicture)
		userGroup.Put("/username", h.ChangeUsername)
	}
}

// GetUser godoc
//
//	@Summary		Get current authenticated user profile
//	@Description	Retrieves the profile information of the currently authenticated user.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Success		200				{object}	model.User				"User profile retrieved successfully.
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during user retrieval"
//	@Router			/user [get]
func (h *UserHandler) GetUser(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	user, err := h.UserService.GetByID(c.Context(), claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(user)
}

// EditUser godoc
//
//	@Summary		Edit current authenticated user profile
//	@Description	Updates the profile information of the currently authenticated user.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.UserEditReq		true	"User profile update payload"
//	@Success		204				{object}	nil						"User profile updated successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request parameters"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during user update"
//	@Router			/user [put]
func (h *UserHandler) EditUser(c fiber.Ctx) error {
	var req model.UserEditReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.UserService.EditByID(c.Context(), claims.UserID, &req)
	if err != nil {
		return err
	}

	return nil
}

// ChangeUsername godoc
//
//	@Summary		Change the username of the authenticated user
//	@Description	Updates the username for the currently authenticated user. The new username must be valid and not already taken.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string					true	"Authorization Bearer token"
//	@Param			request			body		model.UsernameChange	true	"Username change payload"
//	@Success		204				{object}	nil						"Username changed successfully"
//	@Failure		400				{object}	apperrors.ErrorPublic	"Invalid request parameters or invalid username"
//	@Failure		401				{object}	apperrors.ErrorPublic	"Authentication required or invalid token"
//	@Failure		409				{object}	apperrors.ErrorPublic	"This username is already taken"
//	@Failure		500				{object}	apperrors.ErrorPublic	"Internal server error during username change"
//	@Router			/user/username [put]
func (h *UserHandler) ChangeUsername(c fiber.Ctx) error {
	var req model.UsernameChange
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request params")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	err := h.UserService.ChangeUsername(c.Context(), claims.UserID, req.Username)
	if err != nil {
		return err
	}

	return nil
}

// SetProfilePicture godoc
//
//	@Summary		Set profile picture for the authenticated user
//	@Description	Uploads and sets a new profile picture for the currently authenticated user. The image file must be provided as multipart/form-data under the "image" field.
//	@Tags			user
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			Authorization	header		string						true	"Authorization Bearer token"
//	@Param			image			formData	file						true	"Profile picture image file"
//	@Success		200				{object}	object{image_url=string}	"Profile picture updated successfully, returns the new image URL"
//	@Failure		400				{object}	apperrors.ErrorPublic		"Invalid request data or missing image file"
//	@Failure		401				{object}	apperrors.ErrorPublic		"Authentication required or invalid token"
//	@Failure		500				{object}	apperrors.ErrorPublic		"Internal server error during profile picture update"
//	@Router			/user/profile-picture [put]
func (h *UserHandler) SetProfilePicture(c fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	imageURL, err := h.UserService.UpdateProfilePicture(c.Context(), claims.UserID, file)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"image_url": imageURL,
	})
}

// FindUserByUsername godoc
//
//	@Summary		Find user by username
//	@Description	Retrieves user information by the provided username. This endpoint allows public access to view user profiles without authentication.
//	@Tags			user
//	@Accept			json
//	@Produce		json
//	@Param			username	path		string					true	"Username to search for"
//	@Success		200			{object}	model.User				"User information retrieved successfully"
//	@Failure		400			{object}	apperrors.ErrorPublic	"Invalid username format"
//	@Failure		404			{object}	apperrors.ErrorPublic	"User not found"
//	@Failure		500			{object}	apperrors.ErrorPublic	"Internal server error"
//	@Router			/user/username/{username} [get]
func (h *UserHandler) FindUserByUsername(c fiber.Ctx) error {
	username, ok := mtype.NewUsername(c.Params("username"))
	if !ok {
		return apperrors.BadRequest("invalid username provided")
	}

	user, err := h.UserService.GetByUsername(c.Context(), username)
	if err != nil {
		return err
	}

	return c.JSON(user)
}
