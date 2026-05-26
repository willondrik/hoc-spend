package transform

import (
	"fmt"
	"strings"

	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func removeHonorifics(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "Right Hon. ", "")
	s = strings.ReplaceAll(s, "Hon. ", "")
	return strings.TrimSpace(s)
}

// parseCommaName parses names of the form "Last, First".
//
// It also supports a 3-part comma form observed in officer travel pages
// (e.g. "Last Part1, Last Part2, First").
func parseCommaName(name string) types.PersonName {
	name = removeHonorifics(name)
	parts := strings.Split(name, ", ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	if len(parts) == 3 {
		return types.PersonName{FirstName: parts[2], LastName: strings.TrimSpace(parts[0] + " " + parts[1])}
	}
	if len(parts) < 2 {
		return types.PersonName{}
	}
	return types.PersonName{FirstName: parts[1], LastName: parts[0]}
}

// parseTravellerName supports entries like:
//   "Type (Last, First)"
//   "Last, First"
// and strips honorifics.
func parseTravellerName(raw string) (types.PersonName, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.PersonName{}, nil
	}

	// Edge cases observed in data:
	//   "Walsh, Liam, Liam"
	//   "Nath, Arjun, Arjun"
	if strings.Count(raw, ",") >= 2 {
		lastComma := strings.LastIndex(raw, ",")
		if lastComma == -1 {
			return types.PersonName{}, fmt.Errorf("unexpected traveller name: %s", raw)
		}
		raw = strings.TrimSpace(raw[:lastComma])
	}

	if strings.Contains(raw, "(") && strings.Contains(raw, ")") {
		open := strings.IndexByte(raw, '(')
		close := strings.LastIndexByte(raw, ')')
		if open == -1 || close == -1 || close <= open {
			return types.PersonName{}, fmt.Errorf("invalid traveller name: %s", raw)
		}
		inner := strings.TrimSpace(raw[open+1 : close])
		return parseCommaName(inner), nil
	}

	return parseCommaName(raw), nil
}

func parsePlacesVisited(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	return []string{s}
}

