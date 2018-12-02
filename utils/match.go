package utils

import "strings"

// Returns true if keyword is in str
// Handles '*' (0 or more characters) in keyword
func Match(s string, keyword string) bool {
	// Doesn't use regex because of all the escaping that must be done

	// Split the keyword into parts we need to check (that are separated by *)
	kws := strings.Split(keyword, "*")

	cursor := 0
	for _, kw := range kws {
		if kw == "" {
			continue
		}

		// Check that the part is at or after the current cursor position
		// (bc we must keep the order of the parts in the keyword)
		if index := strings.Index(s[cursor:], kw); index >= 0 {
			cursor += index + len(kw)
		} else {
			// keyword not found or found before what we want
			return false
		}
	}

	return true
}
