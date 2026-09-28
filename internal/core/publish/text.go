package publish

import (
	"html"
	"regexp"
	"strings"
)

var (
	tagPattern  = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)
	hrefPattern = regexp.MustCompile(`(?i)\bhref\s*=\s*"([^"]*)"`)
)

// PlainText turns the editor's HTML into the plain text of the networks that
// take no formatting, like Postiz's "normal" editor: paragraphs and list
// items become lines, a link becomes its URL, and bold and underline become
// Unicode letters that look that way.
func PlainText(content string) string {
	var b strings.Builder
	bold, underline, inItem, inLink := 0, 0, 0, false
	write := func(text string) {
		if inLink {
			return
		}
		text = html.UnescapeString(text)
		if bold > 0 {
			text = mapRunes(text, boldRunes)
		}
		if underline > 0 {
			text = underlined(text)
		}
		if inItem > 0 {
			text = strings.ReplaceAll(text, "\n", "")
		}
		b.WriteString(text)
	}
	rest := content
	for {
		loc := tagPattern.FindStringSubmatchIndex(rest)
		if loc == nil {
			write(rest)
			break
		}
		write(rest[:loc[0]])
		closing := rest[loc[2]:loc[3]] == "/"
		name := strings.ToLower(rest[loc[4]:loc[5]])
		attrs := rest[loc[6]:loc[7]]
		rest = rest[loc[1]:]
		switch name {
		case "strong", "b":
			bold += step(closing)
		case "u":
			underline += step(closing)
		case "li":
			inItem += step(closing)
			if closing {
				b.WriteString("\n")
			} else {
				b.WriteString("- ")
			}
		case "p":
			if closing && inItem == 0 {
				b.WriteString("\n")
			}
		case "br":
			b.WriteString("\n")
		case "a":
			if closing {
				inLink = false
				continue
			}
			if href := hrefPattern.FindStringSubmatch(attrs); href != nil {
				b.WriteString(html.UnescapeString(href[1]))
				inLink = true
			}
		}
	}
	return strings.TrimRight(b.String(), "\n ")
}

func step(closing bool) int {
	if closing {
		return -1
	}
	return 1
}

func mapRunes(text string, table map[rune]rune) string {
	return strings.Map(func(r rune) rune {
		if mapped, ok := table[r]; ok {
			return mapped
		}
		return r
	}, text)
}

// underlined adds a combining low line to letters and digits.
func underlined(text string) string {
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune('̲')
		}
	}
	return b.String()
}

// boldRunes are the mathematical sans-serif bold letters and digits Postiz uses.
var boldRunes = func() map[rune]rune {
	table := map[rune]rune{}
	for i := range rune(26) {
		table['a'+i] = 0x1D5EE + i
		table['A'+i] = 0x1D5D4 + i
	}
	for i := range rune(10) {
		table['0'+i] = 0x1D7EC + i
	}
	return table
}()

// linkedInReserved are the characters of LinkedIn's little text format that
// must be escaped to show as themselves.
var linkedInReserved = strings.NewReplacer(
	`\`, `\\`, `<`, `\<`, `>`, `\>`, `#`, `\#`, `~`, `\~`, `_`, `\_`, `|`, `\|`,
	`[`, `\[`, `]`, `\]`, `*`, `\*`, `(`, `\(`, `)`, `\)`, `{`, `\{`, `}`, `\}`, `@`, `\@`,
)

// LinkedInCommentary is the text of a LinkedIn post or comment.
func LinkedInCommentary(content string) string {
	return linkedInReserved.Replace(PlainText(content))
}

// LinkedInURL is the public link to a post.
func LinkedInURL(urn string) string {
	return "https://www.linkedin.com/feed/update/" + urn + "/"
}
