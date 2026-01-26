package util

import "strings"

func NormalizeText(input string) string {
	lower := strings.ToLower(input)
	lower = strings.ReplaceAll(lower, "ł", "l")
	lower = strings.ReplaceAll(lower, "ą", "a")
	lower = strings.ReplaceAll(lower, "ę", "e")
	lower = strings.ReplaceAll(lower, "ś", "s")
	lower = strings.ReplaceAll(lower, "ć", "c")
	lower = strings.ReplaceAll(lower, "ń", "n")
	lower = strings.ReplaceAll(lower, "ó", "o")
	lower = strings.ReplaceAll(lower, "ż", "z")
	lower = strings.ReplaceAll(lower, "ź", "z")
	return lower
}

func ContainsAny(input string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(input, keyword) {
			return true
		}
	}
	return false
}
