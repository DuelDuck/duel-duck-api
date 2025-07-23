package v1

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"gitlab.com/duel-duck/duel-duck-api/internal/model"
	"gitlab.com/duel-duck/duel-duck-api/internal/service"
	"gitlab.com/duel-duck/duel-duck-api/pkg/apperrors"
	auth "gitlab.com/duel-duck/duel-duck-api/pkg/jwt"
	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

type TaskHandler struct {
	TaskService *service.TaskService
	AuthService *service.AuthService
}

func NewTaskHandler(
	taskService *service.TaskService,
	authService *service.AuthService,
) *TaskHandler {
	return &TaskHandler{
		TaskService: taskService,
		AuthService: authService,
	}
}

func (h *TaskHandler) RegisterRoutes(app *fiber.App, auth *AuthHandler) {
	task := app.Group("/task")
	{
		task.Get("/", h.GetTasks, auth.OptionalAuthMiddleware)
	}

	task.Use(auth.AuthMiddleware)
	{
		task.Post("/claim-reward", h.ClaimReward)
		task.Post("/attach-email", h.CompleteAttachEmail)
	}
}

func (h *TaskHandler) ClaimReward(c fiber.Ctx) error {
	var req model.CompleteTaskReq
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	completedTask := model.NewCompletedTask(claims.UserID, req.TaskID)
	resp, err := h.TaskService.ClaimReward(c.Context(), completedTask)
	if err != nil {
		return err
	}

	return c.JSON(resp)
}

func (h *TaskHandler) CompleteAttachEmail(c fiber.Ctx) error {
	var req model.SignInWithEmail
	if err := c.Bind().JSON(&req); err != nil {
		return apperrors.BadRequest("invalid request data")
	}

	email, ok := mtype.NewEmail(req.Email)
	if !ok {
		return apperrors.BadRequest("invalid email")
	}

	ok, err := h.AuthService.CheckUsersEmailCode(c.Context(), email, req.Code)
	if err != nil {
		return err
	}

	if !ok {
		return apperrors.Unauthorized("code does not match")
	}

	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		return apperrors.Unauthorized("claims not found")
	}

	task := model.NewCompletedTask(claims.UserID, model.TaskSignUp)
	userBalance, err := h.TaskService.CompleteSignInEmail(c.Context(), claims.UserID, email, task)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"user_balance": userBalance})
}

func (h *TaskHandler) GetTasks(c fiber.Ctx) error {
	claims, ok := c.Locals("claims").(auth.TokenClaims)
	if !ok {
		// Since GetTasks is a public endpoint
		// If the user is authorized, we set his id to nil
		claims.UserID = uuid.Nil
	}

	tasks, userStats, err := h.TaskService.GetTasksByUserID(c.Context(), claims.UserID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"tasks": tasks, "stats": userStats})
}
