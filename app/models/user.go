package models

import (
	"github.com/go-playground/validator/v10"
	"time"
)

type User struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Username  string `json:"username" gorm:"unique;type:varchar(20)" validate:"required,min=6,max=32"`
	Password  string `json:"password,omitempty" gorm:"type:varchar(255);" validate:"required,min=6"`
	FullName  string `json:"full_name" gorm:"type:varchar(200);" validate:"required,min=6"`
}

type UserSession struct {
	ID                  uint `gorm:"primary_key"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	UserID              uint      `json:"user_id" gorm:"type:int" validate:"required"`
	Token               string    `json:"token" gorm:"type:varchar(255)" validate:"required"`
	RefreshToken        string    `json:"refresh_token" gorm:"type:varchar(255)" validate:"required"`
	TokenExpired        time.Time `json:"-" validate:"required"`
	RefreshTokenExpired time.Time `json:"-" validate:"required"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Username     string `json:"username" `
	Fullname     string `json:"full_name" `
	Token        string `json:"token" `
	RefreshToken string `json:"refresh_token"`
}

func (l User) Validate() error {
	v := validator.New()
	return v.Struct(l)
}

func (l UserSession) Validate() error {
	v := validator.New()
	return v.Struct(l)
}

func (l LoginRequest) Validate() error {
	v := validator.New()
	return v.Struct(l)
}
