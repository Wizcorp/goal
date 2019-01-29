package api

import "strings"

func Concat(strs ...string) string {
	return strings.Join(strs, " ")
}
