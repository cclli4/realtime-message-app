package response

import "github.com/gofiber/fiber/v2"

type Response struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func SendSuccessResponse(ctx *fiber.Ctx, data any) error {
	return ctx.JSON(Response{
		Message: "success",
		Data:    data,
	})
}

func SendFailureResponse(ctx *fiber.Ctx, httpCode int, message string, data interface{}) error {
	return ctx.Status(httpCode).JSON(Response{Message: message, Data: data})
}
