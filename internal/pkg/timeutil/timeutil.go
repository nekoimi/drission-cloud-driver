package timeutil

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

const layout = "2006-01-02 15:04:05"

var globalLocation *time.Location = time.Local

// SetGlobalLocation 设置全局时区
func SetGlobalLocation(tz string) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	globalLocation = loc
	return nil
}

// LocalTime 自定义时间类型，JSON 序列化为 "2006-01-02 15:04:05" 格式
type LocalTime struct {
	time.Time
}

// Now 返回当前时间（使用全局时区）
func Now() LocalTime {
	return LocalTime{Time: time.Now().In(globalLocation)}
}

// MarshalJSON 实现 JSON 序列化
func (lt LocalTime) MarshalJSON() ([]byte, error) {
	if lt.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, lt.In(globalLocation).Format(layout))), nil
}

// UnmarshalJSON 实现 JSON 反序列化
func (lt *LocalTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		lt.Time = time.Time{}
		return nil
	}
	t, err := time.ParseInLocation(layout, s, globalLocation)
	if err != nil {
		return err
	}
	lt.Time = t
	return nil
}

// Scan 实现 sql.Scanner 接口
func (lt *LocalTime) Scan(value interface{}) error {
	if value == nil {
		lt.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		lt.Time = v.In(globalLocation)
	default:
		return fmt.Errorf("cannot scan %T into LocalTime", value)
	}
	return nil
}

// Value 实现 driver.Valuer 接口
func (lt LocalTime) Value() (driver.Value, error) {
	if lt.IsZero() {
		return nil, nil
	}
	return lt.In(time.UTC), nil
}

// GormDataType 返回 GORM 数据类型
func (LocalTime) GormDataType() string {
	return "timestamptz"
}
