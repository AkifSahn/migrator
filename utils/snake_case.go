package utils

import (
	"regexp"
	"strings"
)

func ToSnakeCase(s string) string {
	re := regexp.MustCompile("([a-z])([A-Z])")
	split := re.ReplaceAllString(s, "${1} ${2}")

	fields := strings.Fields(split)

	return strings.Join(fields, "_")
}
