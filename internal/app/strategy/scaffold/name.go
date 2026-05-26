package scaffold

import (
	"errors"
	"regexp"
	"unicode/utf8"
)

const maxNameLen = 64

var namePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ErrInvalidName 表示策略名称不合法。
var ErrInvalidName = errors.New("invalid strategy name")

// ValidateName 校验策略名称格式与长度。
func ValidateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return ErrInvalidName
	}
	if !namePattern.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}
