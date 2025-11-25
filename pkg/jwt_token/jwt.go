package jwt_token

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/cclli4/realtime-message-app/pkg/env"
	"go.elastic.co/apm"
	"time"
)

type ClaimToken struct {
	Username string `json:"username"`
	Fullname string `json:"fullname"`
	jwt.RegisteredClaims
}

var MapTypeToken = map[string]time.Duration{
	"token":         time.Hour * 3,
	"refresh_token": time.Hour * 72,
}

var jwtSecret = []byte(env.GetEnv("APP_SECRET", ""))

func GenerateToken(ctx context.Context, username string, fullname string, tokenType string, now time.Time) (string, error) {
	span, _ := apm.StartSpan(ctx, "GenerateToken", "jwt")
	defer span.End()

	claimToken := ClaimToken{
		Username: username,
		Fullname: fullname,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    env.GetEnv("APP_NAME", ""),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(MapTypeToken[tokenType])),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claimToken)
	result, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}
	return result, nil
}

func ValidateToken(ctx context.Context, token string) (*ClaimToken, error) {
	span, _ := apm.StartSpan(ctx, "ValidateToken", "jwt")
	defer span.End()

	jwtToken, err := jwt.ParseWithClaims(token, &ClaimToken{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}
	claimToken, ok := jwtToken.Claims.(*ClaimToken)

	if !ok || !jwtToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claimToken, nil
}
