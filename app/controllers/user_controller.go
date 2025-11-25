package controllers

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/cclli4/realtime-message-app/app/models"
	"github.com/cclli4/realtime-message-app/app/repository"
	"github.com/cclli4/realtime-message-app/pkg/jwt_token"
	"github.com/cclli4/realtime-message-app/pkg/response"
	"go.elastic.co/apm"
	"golang.org/x/crypto/bcrypt"
	"log"
	"time"
)

// @Description Register a new user
// @Tags        User Authentication
// @Accept      json
// @Produce     json
// @Param       body body models.User true "User Data"
// @Success     201 {object} models.User "User Created Successfully"
// @Failure		500 {object} response.Response
// @Router      /user/v1/register [post]
func Register(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "Register", "controller")
	defer span.End()
	user := new(models.User)
	err := ctx.BodyParser(user)
	if err != nil {
		errResponse := fmt.Errorf("failed to parse request: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusBadRequest, errResponse.Error(), nil)
	}

	err = user.Validate()
	if err != nil {
		errResponse := fmt.Errorf("failed to validate request: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusBadRequest, errResponse.Error(), nil)
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		errResponse := fmt.Errorf("failed to hash password: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, errResponse.Error(), nil)
	}

	user.Password = string(hashedPass)

	err = repository.InsertNewUser(spanCtx, user)
	if err != nil {
		errResponse := fmt.Errorf("failed to insert new user: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, errResponse.Error(), nil)
	}

	resp := user
	resp.Password = ""

	return response.SendSuccessResponse(ctx, resp)

}

// @Description User login
// @Tags        User Authentication
// @Accept      json
// @Produce     json
// @Param       body body models.LoginRequest true "User Data"
// @Success     200 {object} models.LoginResponse
// @Failure		400 {object} response.Response
// @Failure		404 {object} response.Response
// @Router      /user/v1/login [post]
func Login(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "Login", "controller")
	defer span.End()

	loginReq := new(models.LoginRequest)
	err := ctx.BodyParser(loginReq)
	if err != nil {
		errResponse := fmt.Errorf("failed to parse request: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusBadRequest, errResponse.Error(), nil)
	}

	err = loginReq.Validate()
	if err != nil {
		errResponse := fmt.Errorf("failed to validate request: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusBadRequest, errResponse.Error(), nil)
	}

	user, err := repository.GetUserByUsername(spanCtx, loginReq.Username)
	if err != nil {
		errResponse := fmt.Errorf("failed to find user by username: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusNotFound, errResponse.Error(), nil)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password))
	if err != nil {
		errResponse := fmt.Errorf("invalid password: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, errResponse.Error(), nil)
	}

	token, err := jwt_token.GenerateToken(spanCtx, user.Username, user.FullName, "token", time.Now())
	if err != nil {
		errResponse := fmt.Errorf("failed to generate token: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, errResponse.Error(), nil)
	}

	Refreshtoken, err := jwt_token.GenerateToken(spanCtx, user.Username, user.FullName, "refresh_token", time.Now())
	if err != nil {
		errResponse := fmt.Errorf("failed to generate token: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, errResponse.Error(), nil)
	}

	userSession := &models.UserSession{
		UserID:              user.ID,
		Token:               token,
		RefreshToken:        Refreshtoken,
		TokenExpired:        time.Now().Add(jwt_token.MapTypeToken["token"]),
		RefreshTokenExpired: time.Now().Add(jwt_token.MapTypeToken["refresh_token"]),
	}

	err = repository.InsertNewUserSession(spanCtx, userSession)
	if err != nil {
		errResponse := fmt.Errorf("failed insert user session: %v", err)
		log.Println(errResponse)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, "terjadi kesalahan pada sistem", nil)
	}

	var resp models.LoginResponse
	resp.Token = token
	resp.RefreshToken = Refreshtoken
	resp.Username = user.Username
	resp.Fullname = user.FullName

	return response.SendSuccessResponse(ctx, resp)
}

// @Description Logout
// @Tags        User Authentication
// @Param Authorization header string true "Bearer token" default(Bearer <token>)
// @Success     200 {object} response.Response
// @Failure 	401 {object} response.Response
// @Failure		500 {object} response.Response
// @Router      /user/v1/logout [delete]
func Logout(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "Logout", "controller")
	defer span.End()

	token := ctx.Get("authorization")
	err := repository.DeleteUserSessionByToken(spanCtx, token)
	if err != nil {
		errResponse := fmt.Errorf("failed to delete token: %v", err)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, errResponse.Error(), nil)
	}

	return response.SendSuccessResponse(ctx, fiber.StatusOK)
}

// @Description Refresh Token
// @Tags        User Authentication
// @Param Authorization header string true "Bearer token" default(Bearer <token>)
// @Success     200 {object} object{token=string}
// @Failure		500 {object} response.Response
// @Router      /user/v1/refresh-token [put]
func RefreshToken(ctx *fiber.Ctx) error {
	span, spanCtx := apm.StartSpan(ctx.Context(), "RefreshToken", "controller")
	defer span.End()

	now := time.Now()
	refreshToken := ctx.Get("Authorization")
	username := ctx.Locals("username").(string)
	fullName := ctx.Locals("full_name").(string)

	token, err := jwt_token.GenerateToken(spanCtx, username, fullName, "token", now)
	if err != nil {
		errResponse := fmt.Errorf("failed to generate token: %v", err)
		log.Println(errResponse)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, "terjadi kesalahan pada sistem", nil)
	}

	err = repository.UpdateUserSessionToken(spanCtx, token, now.Add(jwt_token.MapTypeToken["token"]), refreshToken)
	if err != nil {
		errResponse := fmt.Errorf("failed to update token: %v", err)
		log.Println(errResponse)
		return response.SendFailureResponse(ctx, fiber.StatusInternalServerError, "terjadi kesalahan pada sistem", nil)
	}

	return response.SendSuccessResponse(ctx, fiber.Map{
		"token": token,
	})
}
