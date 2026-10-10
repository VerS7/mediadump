package parse

import "regexp"

func URLsFromString(source string) []string {
	return regexp.MustCompile(`https?://[^\s<>"']+`).FindAllString(source, -1)
}
