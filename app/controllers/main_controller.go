package controllers

import (
	"github.com/cclli4/realtime-message-app/pkg/env"
	"github.com/gofiber/fiber/v2"
)

func RenderHello(c *fiber.Ctx) error {
	awsPublicIp := env.GetEnv("AWS_PUBLIC_IP", "")
	return c.Render("index", fiber.Map{"awsPublicIP": awsPublicIp})
}

func RenderRegister(c *fiber.Ctx) error {
	return c.Render("register", fiber.Map{})
}
