package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/cclli4/realtime-message-app/app/controllers"
)

type HttpRouter struct {
}

func (h HttpRouter) InstallRouter(app *fiber.App) {
	group := app.Group("", cors.New(), csrf.New())
	group.Get("/", controllers.RenderHello)
	group.Get("/register", controllers.RenderRegister)
}

func NewHttpRouter() *HttpRouter {
	return &HttpRouter{}
}
