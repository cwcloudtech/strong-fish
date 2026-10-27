// Package markdown turns the small dialect strong-fish's email templates are
// written in into HTML.
//
// Deliberately small rather than a full CommonMark implementation. The
// templates are ours: they live in internal/templates, they are reviewed, and
// what they need is paragraphs, a blockquote for the message being forwarded,
// links, and emphasis. A dependency that also handles tables, footnotes and
// raw HTML would be carrying far more than this, and raw HTML is precisely
// what must not pass through: a member's message is interpolated into these
// templates, so everything is escaped first and only the marks below are
// turned back into tags.
//
// What it understands, and nothing else:
//
//	# heading            - a line, extracted by Split rather than rendered
//	paragraph text       - blank-line separated
//	> quoted text        - one or more lines, joined into a blockquote
//	[label](https://...) - a link; only http and https survive
//	**bold**, *italic*
//
// Anything else arrives as literal text, which is the safe way to be wrong.
package markdown

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	linkPattern   = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	boldPattern   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicPattern = regexp.MustCompile(`\*([^*]+)\*`)
)

// Split takes the subject off the top of a rendered template.
//
// The first `# ` line is the email's subject, so a translator editing one file
// changes both the heading and what lands in the inbox, instead of one of them
// drifting from the other in a Go constant they never see.
func Split(document string) (subject, body string) {
	lines := strings.Split(strings.TrimSpace(document), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")),
				strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		}
		break
	}
	return "", strings.TrimSpace(document)
}

// HTML renders the body.
func HTML(body string) string {
	var out strings.Builder
	for _, block := range blocks(body) {
		if quoted, ok := blockquote(block); ok {
			fmt.Fprintf(&out, `<blockquote style="%s">%s</blockquote>`, quoteStyle, inline(quoted))
			continue
		}
		fmt.Fprintf(&out, "<p>%s</p>", inline(block))
	}
	return out.String()
}

// quoteStyle makes a forwarded message look like one: indented, with a rule
// down its left edge. Inline because an email cannot rely on a stylesheet.
const quoteStyle = "margin:12px 0;padding:8px 14px;border-left:3px solid #0e5e9b;" +
	"background-color:#f4f6f9;color:#334155;"

// blocks splits on blank lines, dropping the empties.
func blocks(body string) []string {
	parts := []string{}
	for _, block := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n") {
		if trimmed := strings.TrimSpace(block); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// blockquote recognizes a block whose every line is quoted, and hands back the
// text without the markers.
func blockquote(block string) (string, bool) {
	lines := strings.Split(block, "\n")
	quoted := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ">") {
			return "", false
		}
		quoted = append(quoted, strings.TrimSpace(strings.TrimPrefix(trimmed, ">")))
	}
	return strings.Join(quoted, "\n"), true
}

// inline escapes the text and then puts back the marks it understands.
//
// The order matters: escaping first is what makes a message containing
// <script> arrive as text. The patterns below then match on the escaped
// string, where no < or > of a member's own can be mistaken for a tag.
func inline(text string) string {
	escaped := html.EscapeString(text)

	escaped = linkPattern.ReplaceAllStringFunc(escaped, func(match string) string {
		parts := linkPattern.FindStringSubmatch(match)
		label, href := parts[1], parts[2]
		// A link's target is the one place a template's text becomes an
		// instruction to the mail client. Only the two schemes that mean "a
		// page" are let through; anything else is left as the literal text it
		// was written as.
		if !strings.HasPrefix(href, "https://") && !strings.HasPrefix(href, "http://") {
			return match
		}
		return fmt.Sprintf(`<a href="%s" style="color:#0e5e9b;">%s</a>`, href, label)
	})

	escaped = boldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
	escaped = italicPattern.ReplaceAllString(escaped, "<em>$1</em>")
	// A single newline inside a paragraph is a line break, which is what
	// somebody writing a template expects of a line they deliberately ended.
	return strings.ReplaceAll(escaped, "\n", "<br>")
}
