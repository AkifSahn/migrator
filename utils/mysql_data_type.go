package utils

import (
	"fmt"
	"strings"
)

func ToMysqlDataType(s string) (string, error) {

	switch strings.TrimSpace(s) {
	case "string":
		return "varchar(255)", nil
	case "int", "int32", "int64":
		return "bigint", nil
	case "uint", "uint32", "uint64":
		return "bigint unsigned", nil
	case "float32":
		return "float", nil
	case "float64":
		return "double", nil
	case "time.Time":
		return "datetime(3)", nil
	case "bool":
		return "tinyint(1)", nil
	}
	return "", fmt.Errorf("Cannot convert %s to mysql type, explicit definition in tags required!", s)
}
