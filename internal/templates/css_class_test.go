package templates

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// classWithoutARule names a class the templates wear that the stylesheet has
// never heard of, and says why it is allowed to stay that way for now. An
// entry here is a promise that somebody looked, not a way to quieten the test.
// classWithoutARule names a class the templates wear that the stylesheet has
// never heard of, and says why it is allowed to stay that way. An entry here
// is a promise that somebody looked, not a way to quieten the test.
//
// Empty, and worth keeping empty.
var classWithoutARule = map[string]string{}

// TestEveryClassInAMarkupFileHasARule is the guard against a class name that
// does nothing.
//
// .fail was the case it is named after: every form error message in the
// application carried it, in seven templates, and app.css had no rule for it
// in any revision — so a refused sign-in said "Das passt nicht" in the same
// colour as the sentence explaining the field above it. Nothing was broken in
// a way a test could see, because nothing was wrong with the markup: it asked
// for a state and the stylesheet silently declined. The same had happened to
// .error and .note, which are what two other templates called the same two
// states, and to .tag, which is what one page called a badge the match list
// already had a rule for.
//
// So the rule is mechanical: a class named in a template has to be mentioned
// in the stylesheet. Mentioned, not matched — .set .score-field counts for
// score-field, because a guard that insisted on a bare single-class rule would
// argue with every rule that is properly scoped.
//
// Read off the sources rather than rendered markup, so every template is
// covered without any of them having to be built here. Only literal
// class="..." attributes: a computed class list (templ.KV and friends) is
// spelled in Go and belongs to whatever builds it.
func TestEveryClassInAMarkupFileHasARule(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("..", "..", "web", "static", "css", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	// Comments first. They describe classes that were renamed away as often as
	// they describe the ones below them, and a rule that exists only in prose
	// is exactly what this test is looking for.
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(string(css), "")

	templates, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 {
		t.Fatal("no templates found; the test is looking in the wrong place")
	}

	attribute := regexp.MustCompile(`class="([^"{}]*)"`)
	for _, path := range templates {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(source), "\n") {
			// A templ comment is Go's, and the markup inside one is not
			// rendered. Several of them quote the attribute they replaced.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, m := range attribute.FindAllStringSubmatch(line, -1) {
				for _, class := range strings.Fields(m[1]) {
					if _, known := classWithoutARule[class]; known {
						continue
					}
					if mentions(stripped, class) {
						continue
					}
					t.Errorf("%s: .%s is worn in the markup and has no rule in app.css — "+
						"the class says something the page does not show", path, class)
				}
			}
		}
	}
}

// mentions reports whether the stylesheet names this class anywhere, in any
// selector. The boundary is what keeps .card from being satisfied by
// .recovery-card and .fail from being satisfied by .failure-note.
func mentions(css, class string) bool {
	for i := 0; ; {
		at := strings.Index(css[i:], "."+class)
		if at < 0 {
			return false
		}
		i += at + len(class) + 1
		if i >= len(css) || !isClassChar(css[i]) {
			return true
		}
	}
}

func isClassChar(b byte) bool {
	return b == '-' || b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
