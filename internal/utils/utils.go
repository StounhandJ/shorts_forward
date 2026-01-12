package utils

import (
	"errors"
	"math/rand"
	"regexp"
)

func RandomElement[T any](s []T) (T, error) {
	var zero T
	if len(s) == 0 {
		return zero, errors.New("slice is empty")
	}

	return s[rand.Intn(len(s))], nil
}

func StringNotEmptyCoalesce(args ...string) string {
	for _, elem := range args {
		if len(elem) > 0 {
			return elem
		}
	}

	return ""
}

func SanitizeFileName(name string) string {
	// Replace invalid Windows characters with underscores
	re := regexp.MustCompile(`[\/\?<>\\:\*\|"]`)

	return re.ReplaceAllString(name, "_")
}
