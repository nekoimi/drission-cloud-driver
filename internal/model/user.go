package model

import (
	"github.com/nekoimi/go-project-template/internal/pkg/snowflake"
	"github.com/nekoimi/go-project-template/internal/pkg/timeutil"
	"gorm.io/gorm"
)

type User struct {
	ID        int64             `gorm:"primaryKey;type:bigint" json:"id,string"`
	Username  string            `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Email     string            `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password  string            `gorm:"type:varchar(255);not null" json:"-"`
	CreatedAt timeutil.LocalTime `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt timeutil.LocalTime `gorm:"autoUpdateTime" json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == 0 {
		u.ID = snowflake.GenerateID().Int64()
	}
	return nil
}
