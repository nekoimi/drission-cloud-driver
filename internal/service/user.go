package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/nekoimi/go-project-template/internal/dto"
	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/idutil"
	"github.com/nekoimi/go-project-template/internal/repository"
)

type UserService interface {
	GetProfile(ctx context.Context, userID string) (*dto.UserResponse, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetProfile(ctx context.Context, userID string) (*dto.UserResponse, error) {
	uid, err := idutil.ParseSnowflakeID(userID)
	if err != nil {
		return nil, errcode.New(errcode.Unauthorized)
	}

	user, err := s.userRepo.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrUserNotFound)
		}
		return nil, err
	}

	return &dto.UserResponse{
		ID:        idutil.FormatSnowflakeID(user.ID),
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}
