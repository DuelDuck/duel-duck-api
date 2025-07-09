package swagger

import (
	_ "embed"

	"github.com/gofiber/fiber/v3"
	"gitlab.com/duel-duck/duel-duck-api/config"
)

type Handler struct {
	SwaggerUIURL string
}

func NewSwaggerHandler(c *config.Config) *Handler {
	return &Handler{
		SwaggerUIURL: c.HTTP.SwaggerValidatorURL + c.HTTP.Address + "/swagger.json",
	}
}

//go:embed swagger.json
var swaggerJSON []byte

func (h *Handler) RegisterRoutes(app *fiber.App) {
	app.Get("/swagger.json", func(c fiber.Ctx) error {
		return c.Send(swaggerJSON)
	})

	app.Get("/docs", func(c fiber.Ctx) error {
		return c.Redirect().To(h.SwaggerUIURL)
	})
}
