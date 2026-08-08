package spec

import (
	"fmt"
	"strings"
)

type Complaint struct {
	Question string
}

func ParseComplaintMarkdown(input string) (Complaint, error) {
	sections := parseMarkdownSections(input)
	if question, ok := sections["question"]; ok {
		question = strings.TrimSpace(question)
		if question == "" {
			return Complaint{}, fmt.Errorf("empty Question section")
		}
		return Complaint{
			Question: question,
		}, nil
	}
	question := strings.TrimSpace(normalizeLineEndings(input))
	if question == "" {
		return Complaint{}, fmt.Errorf("empty complaint")
	}
	return Complaint{
		Question: question,
	}, nil
}

func ComplaintMarkdown(c Complaint) string {
	return strings.TrimSpace(fmt.Sprintf(`# Question

%s
`, strings.TrimSpace(c.Question))) + "\n"
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
