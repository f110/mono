package issue

import (
	"regexp"
	"strconv"
	"strings"

	"go.f110.dev/mono/go/issue/database"
)

// Reference is the reference to the ticket of Number in the trailer of the commit message.
type Reference struct {
	Kind   database.TrailerKind
	Number int32
}

var trailerLine = regexp.MustCompile(`(?i)^(refs|fixes):\s*#?(\d+)\s*$`)

// ParseTrailer returns the references written as "Refs: <number>" or "Fixes: <number>" in the last paragraph of message.
// The number may be prefixed with #. The subject is not the trailer even if the message has only one paragraph.
func ParseTrailer(message string) []Reference {
	paragraphs := strings.Split(strings.TrimSpace(strings.ReplaceAll(message, "\r\n", "\n")), "\n\n")
	if len(paragraphs) < 2 {
		return nil
	}
	return parseReferenceLines(paragraphs[len(paragraphs)-1])
}

// ParseReferences returns the references written as "Refs: <number>" or "Fixes: <number>" in any line of body such as
// the body of the pull request. The reference in the middle of the line is ignored.
func ParseReferences(body string) []Reference {
	return parseReferenceLines(strings.ReplaceAll(body, "\r\n", "\n"))
}

func parseReferenceLines(s string) []Reference {
	var refs []Reference
	for _, l := range strings.Split(s, "\n") {
		m := trailerLine.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			continue
		}
		n, err := strconv.ParseInt(m[2], 10, 32)
		if err != nil {
			continue
		}
		kind := database.TrailerKindRefs
		if strings.EqualFold(m[1], "fixes") {
			kind = database.TrailerKindFixes
		}
		refs = append(refs, Reference{Kind: kind, Number: int32(n)})
	}
	return refs
}
