package service

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/nekoimi/go-project-template/internal/model"
	"github.com/nekoimi/go-project-template/internal/pkg/errcode"
	"github.com/nekoimi/go-project-template/internal/pkg/idutil"
	"github.com/nekoimi/go-project-template/internal/repository"
)

type stubUserRepo struct {
	user *model.User
	err  error
}

func (s *stubUserRepo) Create(ctx context.Context, user *model.User) error { return nil }

func (s *stubUserRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

func (s *stubUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubUserRepo) WithTx(tx *gorm.DB) repository.UserRepository { return s }

func TestUserService_GetProfile_IDAsString(t *testing.T) {
	t.Parallel()

	id := int64(1234567890123456789)
	repo := &stubUserRepo{
		user: &model.User{
			ID:       id,
			Username: "u",
			Email:    "e@e.com",
		},
	}
	svc := NewUserService(repo)

	got, err := svc.GetProfile(context.Background(), idutil.FormatSnowflakeID(id))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "1234567890123456789" {
		t.Fatalf("ID = %q, want decimal string", got.ID)
	}
}

func TestUserService_GetProfile_InvalidIDUnauthorized(t *testing.T) {
	t.Parallel()

	svc := NewUserService(&stubUserRepo{user: &model.User{}})
	_, err := svc.GetProfile(context.Background(), "not-a-number")
	app, ok := err.(*errcode.AppError)
	if !ok || app.Code != errcode.Unauthorized {
		t.Fatalf("err = %v, want AppError Unauthorized", err)
	}
}
