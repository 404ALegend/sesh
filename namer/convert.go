package namer

import "strings"

var (
	defaultNameReplacer = strings.NewReplacer(".", "_", ":", "_")
	tuiosNameReplacer   = strings.NewReplacer(".", "_", ":", "_", "%", "%25", "/", "%2F", `\`, "%5C")
)

func convertToValidName(name, backend string) string {
	replacer := defaultNameReplacer
	if backend == "tuios" {
		// TUIOS uses the session name as a state filename. Escape the
		// separators and the escape marker so their spellings stay distinct.
		replacer = tuiosNameReplacer
	}
	return strings.Join(strings.Fields(replacer.Replace(name)), "_")
}
