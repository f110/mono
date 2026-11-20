package git

import (
	"regexp"
	"strings"
)

type attributeRule struct {
	re *regexp.Regexp
	// generated is the value of linguist-generated. false also means unset.
	generated bool
}

// Attributes is the parsed .gitattributes. Only linguist-generated is supported.
type Attributes struct {
	rules []attributeRule
}

// ParseAttributes parses .gitattributes at the root of the repository. The lines which don't have linguist-generated
// and the invalid lines are ignored.
func ParseAttributes(content []byte) *Attributes {
	f := &Attributes{}
	for line := range strings.Lines(string(content)) {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		var generated, found bool
		for _, attr := range fields[1:] {
			switch attr {
			case "linguist-generated", "linguist-generated=true":
				generated, found = true, true
			case "-linguist-generated", "!linguist-generated", "linguist-generated=false":
				generated, found = false, true
			}
		}
		if !found {
			continue
		}
		re, ok := compileAttributePattern(fields[0])
		if !ok {
			continue
		}
		f.rules = append(f.rules, attributeRule{re: re, generated: generated})
	}
	return f
}

// Generated reports whether the file is marked as linguist-generated. path is relative to the root of the repository.
// The last matching line wins like git.
func (f *Attributes) Generated(path string) bool {
	generated := false
	for _, r := range f.rules {
		if r.re.MatchString(path) {
			generated = r.generated
		}
	}
	return generated
}

// compileAttributePattern converts the pattern of gitattributes into the regular expression which matches the whole
// path. The pattern without the slash matches the name at any depth, and the other patterns are relative to the root.
// The pattern ending with the slash never matches the file and the negative pattern is forbidden in gitattributes.
func compileAttributePattern(pattern string) (*regexp.Regexp, bool) {
	if strings.HasSuffix(pattern, "/") || strings.HasPrefix(pattern, "!") {
		return nil, false
	}
	anchored := strings.Contains(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	var b strings.Builder
	b.WriteString("^")
	if !anchored {
		b.WriteString("(?:.*/)?")
	}
	for i := 0; i < len(pattern); {
		rest := pattern[i:]
		switch {
		case strings.HasPrefix(rest, "**/") && (i == 0 || pattern[i-1] == '/'):
			b.WriteString("(?:.*/)?")
			i += 3
		case rest == "**" && i > 0 && pattern[i-1] == '/':
			b.WriteString(".*")
			i += 2
		case strings.HasPrefix(rest, "**"):
			b.WriteString("[^/]*")
			i += 2
		case rest[0] == '*':
			b.WriteString("[^/]*")
			i++
		case rest[0] == '?':
			b.WriteString("[^/]")
			i++
		case rest[0] == '[':
			end := strings.IndexByte(rest[1:], ']')
			if end < 0 {
				return nil, false
			}
			class := rest[1 : end+1]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteByte('[')
			b.WriteString(strings.ReplaceAll(class, `\`, `\\`))
			b.WriteByte(']')
			i += end + 2
		case rest[0] == '\\' && len(rest) > 1:
			b.WriteString(regexp.QuoteMeta(rest[1:2]))
			i += 2
		default:
			b.WriteString(regexp.QuoteMeta(rest[:1]))
			i++
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false
	}
	return re, true
}
