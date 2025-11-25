package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/cclli4/realtime-message-app/app/repository"
	"github.com/cclli4/realtime-message-app/pkg/response"
	"go.elastic.co/apm"
	"log"
)

// @Description Get All Messsage History
// @Tags        Message
// @Param 		Authorization header string true "Bearer token" default(Bearer <token>)
// @Success 200 {array} models.MessagePayload
// @Failure		500 {object} response.Response
// @Router      /message/v1/history [get]
func GetHistory(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "GetHistory", "controller")
	defer span.End()

	resp, err := repository.GetAllMessage(spanCtx)
	if err != nil {
		log.Println(err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, "terjadi kesalahan pada server", nil)
	}
	return response.SendSuccessResponse(ctx, resp)
}
