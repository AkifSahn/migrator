package utils

import "strings"

// Pluralize converts a given string to its plural form if it is not already plural.
func Pluralize(word string) string {
	// Convert word to lowercase for consistent comparisons
	lowered := strings.ToLower(word)

	// Check for common plural forms and return the word if it's already plural
	if strings.HasSuffix(lowered, "s") || strings.HasSuffix(lowered, "es") || strings.HasSuffix(lowered, "ies") {
		return word
	}

	// Handle specific rules for pluralization
	if strings.HasSuffix(lowered, "y") && !isVowel(lowered[len(lowered)-2]) {
		// If the word ends in 'y' and the letter before it is not a vowel, replace 'y' with 'ies'
		return word[:len(word)-1] + "ies"
	} else if strings.HasSuffix(lowered, "o") || strings.HasSuffix(lowered, "ch") || strings.HasSuffix(lowered, "sh") {
		// Words ending in 'o', 'ch', or 'sh' usually get 'es' for plural
		return word + "es"
	} else if strings.HasSuffix(lowered, "f") || strings.HasSuffix(lowered, "fe") {
		// Words ending in 'f' or 'fe' typically get 'ves' for plural
		if strings.HasSuffix(lowered, "fe") {
			return word[:len(word)-2] + "ves"
		}
		return word[:len(word)-1] + "ves"
	}

	// Default rule: just add 's' to the end of the word
	return word + "s"
}

// Helper function to check if a character is a vowel
func isVowel(c byte) bool {
	vowels := "aeiou"
	return strings.ContainsRune(vowels, rune(c))
}

func ToMysqlName(word string) string {
	return strings.ToLower(ToSnakeCase(word))
}
