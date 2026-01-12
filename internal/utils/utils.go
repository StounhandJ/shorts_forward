package utils

import (
	"errors"
	"fmt"
	"math/rand"
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

func FormatSecondsToMMSS(sec int) string {
	minutes := sec / 60
	seconds := sec % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func FormatBigInt(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000.0)
	case n < 1_000_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000.0)
	default:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000.0)
	}
}
