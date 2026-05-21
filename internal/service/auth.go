package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/nekoimi/go-project-template/internal/dto"
	"github.com/nekoimi/go-project-template/internal/model"
	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/idutil"
	"github.com/nekoimi/go-project-template/internal/repository"
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
}

type authService struct {
	userRepo  repository.UserRepository
	db        *gorm.DB
	jwtSecret string
	jwtExpire time.Duration
}

func NewAuthService(userRepo repository.UserRepository, db *gorm.DB, jwtSecret string, jwtExpire time.Duration) AuthService {
	return &authService{
		userRepo:  userRepo,
		db:        db,
		jwtSecret: jwtSecret,
		jwtExpire: jwtExpire,
	}
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashed),
	}

	// 使用事务包裹唯一性检查和创建，防止并发竞态
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := s.userRepo.WithTx(tx)

		if _, err := txRepo.FindByEmail(ctx, req.Email); err == nil {
			return errcode.New(errcode.ErrEmailExists)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if _, err := txRepo.FindByUsername(ctx, req.Username); err == nil {
			return errcode.New(errcode.ErrUsernameExists)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		return txRepo.Create(ctx, user)
	})
	if err != nil {
		return nil, err
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		Token: token,
		User: dto.UserInfo{
			ID:        idutil.FormatSnowflakeID(user.ID),
			Username:  user.Username,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrInvalidCreds)
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errcode.New(errcode.ErrInvalidCreds)
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		Token: token,
		User: dto.UserInfo{
			ID:        idutil.FormatSnowflakeID(user.ID),
			Username:  user.Username,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}

func (s *authService) generateToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": idutil.FormatSnowflakeID(userID),
		"exp": time.Now().Add(s.jwtExpire).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
