package modelcatalog

import (
	"regexp"
	"sort"
	"strings"

	"github.com/lihaozhe/pie-tui/internal/config"
)

// Resolution is how one enabledModels pattern resolves against the catalog.
type Resolution struct {
	Pattern string
	Matched []string
	Pin     string
}

// EnabledSet resolves enabledModels. hasPatterns false means every model is
// enabled. Pins maps a reference to the thinking level its pattern fixes.
func EnabledSet(patterns []string, patternsSet bool, models []*Model) (enabled map[string]bool, pins map[string]string, resolutions []Resolution) {
	enabled = make(map[string]bool, len(models))
	pins = map[string]string{}
	for _, model := range models {
		enabled[model.Ref()] = false
	}
	if !patternsSet {
		for _, model := range models {
			enabled[model.Ref()] = true
		}
		return enabled, pins, nil
	}
	for _, pattern := range patterns {
		base, pin := splitPin(pattern)
		resolution := Resolution{Pattern: pattern, Pin: pin}
		for _, model := range models {
			if matchPattern(base, model) {
				resolution.Matched = append(resolution.Matched, model.Ref())
				enabled[model.Ref()] = true
				if pin != "" {
					pins[model.Ref()] = pin
				}
			}
		}
		resolutions = append(resolutions, resolution)
	}
	return enabled, pins, resolutions
}

// StalePatterns lists configured patterns that match no model in the catalog.
func StalePatterns(resolutions []Resolution) []string {
	var stale []string
	for _, resolution := range resolutions {
		if len(resolution.Matched) == 0 {
			stale = append(stale, resolution.Pattern)
		}
	}
	return stale
}

// SetEnabled turns one model on or off with a minimal edit of the pattern list,
// the same policy pie follows: a pattern that covers the model is expanded in
// place, and unmatched patterns are never touched.
func SetEnabled(patterns []string, patternsSet bool, models []*Model, ref string, on bool) ([]string, bool) {
	enabled, pins, resolutions := EnabledSet(patterns, patternsSet, models)
	if on == enabled[ref] {
		return patterns, patternsSet
	}

	if on {
		if !patternsSet {
			return patterns, true
		}
		next := append([]string(nil), patterns...)
		entry := ref
		if pin := pins[ref]; pin != "" {
			entry += ":" + pin
		}
		return append(next, entry), true
	}

	if !patternsSet {
		// Materialize "everything" as explicit references, then drop one.
		out := make([]string, 0, len(models))
		for _, model := range models {
			if model.Ref() == ref {
				continue
			}
			out = append(out, model.Ref())
		}
		return out, true
	}

	next := make([]string, 0, len(patterns)+4)
	for _, resolution := range resolutions {
		contains := false
		for _, matched := range resolution.Matched {
			if matched == ref {
				contains = true
				break
			}
		}
		if !contains {
			next = append(next, resolution.Pattern)
			continue
		}
		// Expand in place: every other model the pattern covered keeps its pin.
		for _, matched := range resolution.Matched {
			if matched == ref {
				continue
			}
			entry := matched
			if resolution.Pin != "" {
				entry += ":" + resolution.Pin
			}
			next = append(next, entry)
		}
	}
	return next, true
}

// SetAllEnabled returns the pattern list for "every model on": no patterns.
func SetAllEnabled() ([]string, bool) { return nil, false }

// SetAllDisabled returns one kept reference, because pi needs a model to start.
func SetAllDisabled(models []*Model) ([]string, bool) {
	if len(models) == 0 {
		return []string{}, true
	}
	return []string{models[0].Ref()}, true
}

func splitPin(pattern string) (string, string) {
	index := strings.LastIndex(pattern, ":")
	if index <= 0 || index == len(pattern)-1 {
		return pattern, ""
	}
	level := pattern[index+1:]
	if !config.IsThinkingLevel(level) {
		return pattern, ""
	}
	return pattern[:index], level
}

var globChars = strings.NewReplacer(
	"\\", "\\\\",
	".", "\\.",
	"+", "\\+",
	"(", "\\(",
	")", "\\)",
	"|", "\\|",
	"^", "\\^",
	"$", "\\$",
	"[", "\\[",
	"]", "\\]",
	"{", "\\{",
	"}", "\\}",
)

// matchPattern accepts an exact reference, a glob, or a case-insensitive fuzzy
// substring of the reference or model name. A glob's * crosses "/" so
// provider/* covers model ids that contain slashes, as pie ensures with
// verified provider globs.
func matchPattern(pattern string, model *Model) bool {
	ref := model.Ref()
	if pattern == ref {
		return true
	}
	if strings.ContainsAny(pattern, "*?") {
		expression := globChars.Replace(pattern)
		expression = strings.ReplaceAll(expression, "**", "\x00")
		expression = strings.ReplaceAll(expression, "*", ".*")
		expression = strings.ReplaceAll(expression, "\x00", ".*")
		expression = strings.ReplaceAll(expression, "?", ".")
		re, err := regexp.Compile("(?i)^" + expression + "$")
		if err == nil && re.MatchString(ref) {
			return true
		}
		re, err = regexp.Compile("(?i)^" + expression + "$")
		if err == nil && re.MatchString(model.Name) {
			return true
		}
		return false
	}
	needle := strings.ToLower(pattern)
	if strings.Contains(strings.ToLower(ref), needle) {
		return true
	}
	return model.Name != "" && strings.Contains(strings.ToLower(model.Name), needle)
}

// EnabledRefs returns enabled references in catalog order.
func EnabledRefs(models []*Model, enabled map[string]bool) []string {
	refs := make([]string, 0, len(enabled))
	for _, model := range models {
		if enabled[model.Ref()] {
			refs = append(refs, model.Ref())
		}
	}
	return refs
}

// SortRefs orders references by provider then model id.
func SortRefs(refs []string) []string {
	sorted := append([]string(nil), refs...)
	sort.Strings(sorted)
	return sorted
}
