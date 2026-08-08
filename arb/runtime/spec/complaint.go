package spec

import (
	"fmt"
	"strings"
)

type Complaint struct {
	Proposition string
}

func ParseComplaintMarkdown(input string) (Complaint, error) {
	sections := parseMarkdownSections(input)
	if proposition, ok := sections["proposition"]; ok {
		proposition = strings.TrimSpace(proposition)
		if proposition == "" {
			return Complaint{}, fmt.Errorf("empty Proposition section")
		}
		return Complaint{
			Proposition: proposition,
		}, nil
	}
	proposition := strings.TrimSpace(normalizeLineEndings(input))
	if proposition == "" {
		return Complaint{}, fmt.Errorf("empty complaint")
	}
	return Complaint{
		Proposition: proposition,
	}, nil
}

func ComplaintMarkdown(c Complaint) string {
	return strings.TrimSpace(fmt.Sprintf(`# Proposition

%s
`, strings.TrimSpace(c.Proposition))) + "\n"
}

func parseMarkdownSections(input string) map[string]string {
	sections := map[string]string{}
	var current string
	var body []string
	flush := func() {
		if current == "" {
			return
		}
		sections[current] = strings.TrimSpace(strings.Join(body, "\n"))
	}
	for _, raw := range strings.Split(normalizeLineEndings(input), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") {
			flush()
			current = normalizeHeading(line)
			body = body[:0]
			continue
		}
		if current != "" {
			body = append(body, raw)
		}
	}
	flush()
	return sections
}

func normalizeHeading(line string) string {
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(strings.ToLower(line))
	return line
}

func normalizeLineEndings(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	return strings.ReplaceAll(input, "\r", "\n")
}
