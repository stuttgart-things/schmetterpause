package templates

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoTableCellWearsAClassThatSetsDisplay is the guard against the bug it is
// named after.
//
// The match list heads its set-count cell <td class="num score">, and result
// entry wrapped its box and steps in <span class="score">. One of the two then
// grew a rule with `display: inline-flex` in it, and the other silently
// stopped being a table cell: it no longer filled its row, so its bottom
// border sat halfway up the row and the line across the table broke wherever a
// neighbour wrapped onto a second line. On a phone, where names wrap, that was
// every other row.
//
// Nothing about either name was wrong. What was wrong is that a class which
// sets `display` is a layout claim, and a table cell already has a display the
// table depends on. So the rule is narrow and mechanical: a class named in a
// td or th may not be one that sets display anywhere in the stylesheet.
//
// Read off the sources rather than rendered markup, because every template
// with a table is covered without any of them having to be built here.
func TestNoTableCellWearsAClassThatSetsDisplay(t *testing.T) {
	layout := classesThatSetDisplay(t)

	cell := regexp.MustCompile(`<t[dh] class="([^"]*)"`)
	templates, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 {
		t.Fatal("no templates found; the test is looking in the wrong place")
	}

	for _, path := range templates {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cell.FindAllStringSubmatch(string(source), -1) {
			for _, class := range strings.Fields(m[1]) {
				if layout[class] {
					t.Errorf("%s: a table cell wears .%s, and .%s sets display — "+
						"the cell stops being a table cell and its border leaves the row",
						path, class, class)
				}
			}
		}
	}
}

// classesThatSetDisplay collects every class that a single-class rule gives a
// display to. Single-class only: `.set .score-field` is scoped to somewhere a
// table cell cannot be, and reading it as a claim on the bare name would make
// the test complain about rules that are already careful.
func classesThatSetDisplay(t *testing.T) map[string]bool {
	t.Helper()

	source, err := os.ReadFile(filepath.Join("..", "..", "web", "static", "css", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	// Comments first: they hold braces and selectors, this one included.
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(source), "")

	rule := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	bare := regexp.MustCompile(`^\.[a-zA-Z0-9_-]+$`)
	display := regexp.MustCompile(`(^|[;{\s])display\s*:`)

	found := map[string]bool{}
	for _, m := range rule.FindAllStringSubmatch(stripped, -1) {
		if !display.MatchString(m[2]) {
			continue
		}
		for _, selector := range strings.Split(m[1], ",") {
			selector = strings.TrimSpace(selector)
			if bare.MatchString(selector) {
				found[strings.TrimPrefix(selector, ".")] = true
			}
		}
	}
	if len(found) == 0 {
		t.Fatal("no class sets display, which cannot be true — the stylesheet did not parse")
	}
	return found
}
