package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/cclli4/realtime-message-app/app/repository"
	"github.com/cclli4/realtime-message-app/pkg/jwt_token"
	"github.com/cclli4/realtime-message-app/pkg/response"
	"go.elastic.co/apm"
	"log"
	"time"
)

func MiddlewareValidateAuth(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "MiddlewareValidateAuth", "middleware")
	defer span.End()

	auth := ctx.Get("authorization")

	if auth == "" {
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	_, err := repository.GetUserSessionByToken(spanCtx, auth)
	if err != nil {
		log.Println("failed to get user session on DB: ", err)
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	claim, err := jwt_token.ValidateToken(spanCtx, auth)
	if err != nil {
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	if time.Now().Unix() > claim.ExpiresAt.Unix() {
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	ctx.Set("username", claim.Username)
	ctx.Set("full_name", claim.Fullname)

	return ctx.Next()

}

func MiddlewareRefreshToken(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "MiddlewareRefreshToken", "middleware")
	defer span.End()

	auth := ctx.Get("authorization")
	if auth == "" {
		log.Println("authorization empty")
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	claim, err := jwt_token.ValidateToken(spanCtx, auth)
	if err != nil {
		log.Println(err)
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	if time.Now().Unix() > claim.ExpiresAt.Unix() {
		log.Println("jwt token is expired: ", claim.ExpiresAt)
		return response.SendFailureResponse(ctx, fiber.StatusUnauthorized, "unauthorized", nil)
	}

	ctx.Locals("username", claim.Username)
	ctx.Locals("full_name", claim.Fullname)

	return ctx.Next()
}
