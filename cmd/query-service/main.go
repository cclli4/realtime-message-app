package main

import (
	"context"
	"log"
	"net/url"
	"time"

	"github.com/cclli4/realtime-message-app/app/repository"
	"github.com/cclli4/realtime-message-app/pkg/database"
	"github.com/cclli4/realtime-message-app/pkg/env"
	"github.com/cclli4/realtime-message-app/pkg/presence"
	"github.com/gofiber/fiber/v2"
)

func main() {
	env.SetupEnvFile()
	database.SetupMongoDB()

	app := fiber.New()

	app.Get("/messages", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		messages, err := repository.GetAllMessage(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(messages)
	})

	redisURL := env.GetEnv("REDIS_URL", "")
	gatewayID := env.GetEnv("GATEWAY_INSTANCE_ID", "query-service")
	var presenceStore *presence.Store
	if redisURL != "" {
		store, err := presence.NewStore(redisURL, gatewayID, 45*time.Second)
		if err != nil {
			log.Printf("query service presence store disabled: %v", err)
		} else {
			presenceStore = store
			defer presenceStore.Close()
		}
	}

	app.Get("/presence/:user", func(c *fiber.Ctx) error {
		if presenceStore == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "presence store not configured",
			})
		}
		rawUser := c.Params("user")
		user, err := url.PathUnescape(rawUser)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid user parameter",
			})
		}

		gateway, err := presenceStore.GetGateway(user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(fiber.Map{
			"user":      user,
			"gateway":   gateway,
			"is_online": gateway != "",
		})
	})

	port := env.GetEnv("QUERY_SERVICE_PORT", "3100")
	if err := app.Listen("0.0.0.0:" + port); err != nil {
		log.Fatalf("query service failed: %v", err)
	}
}
