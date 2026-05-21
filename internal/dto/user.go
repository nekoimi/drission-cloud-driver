package dto

import "github.com/nekoimi/drission-cloud-driver/internal/pkg/timeutil"

type UserResponse struct {
	ID        string             `json:"id"`
	Username  string             `json:"username"`
	Email     string             `json:"email"`
	CreatedAt timeutil.LocalTime `json:"created_at"`
	UpdatedAt timeutil.LocalTime `json:"updated_at"`
}
