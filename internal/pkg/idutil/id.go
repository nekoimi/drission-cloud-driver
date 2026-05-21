package idutil

import (
	"errors"
	"strconv"
)

var (
	ErrInvalidID = errors.New("invalid id")
)

// FormatSnowflakeID converts numeric snowflake ID to string.
func FormatSnowflakeID(id int64) string {
	return strconv.FormatInt(id, 10)
}

// ParseSnowflakeID parses decimal string snowflake ID into int64.
func ParseSnowflakeID(id string) (int64, error) {
	if id == "" {
		return 0, ErrInvalidID
	}

	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, ErrInvalidID
	}

	return parsed, nil
}
