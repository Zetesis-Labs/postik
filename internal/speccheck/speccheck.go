// Package speccheck verifies that every case Sxx.n of the technical specs is
// cited by at least one test, and that no test cites a case that does not exist.
package speccheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
)

var (
	specHeading = regexp.MustCompile(`(?m)^## (S\d{2}\.\d+)\b`)
	caseAtStart = regexp.MustCompile(`^(S\d{2}\.\d+)\b`)
	e2eTitle    = regexp.MustCompile("\\btest\\(\\s*['\"`](S\\d{2}\\.\\d+)\\b")
)

// Sources maps file paths to their contents.
type Sources struct {
	Specs   map[string]string
	GoTests map[string]string
	E2E     map[string]string
}

type Citation struct {
	Case string
	File string
}

type Report struct {
	Untested []string
	Unknown  []Citation
}

func (r Report) OK() bool {
	return len(r.Untested) == 0 && len(r.Unknown) == 0
}

func Check(src Sources) (Report, error) {
	defined := map[string]bool{}
	for _, content := range src.Specs {
		for _, match := range specHeading.FindAllStringSubmatch(content, -1) {
			defined[match[1]] = true
		}
	}

	var citations []Citation
	for file, content := range src.GoTests {
		cases, err := goTestCitations(file, content)
		if err != nil {
			return Report{}, err
		}
		for _, c := range cases {
			citations = append(citations, Citation{Case: c, File: file})
		}
	}
	for file, content := range src.E2E {
		for _, match := range e2eTitle.FindAllStringSubmatch(content, -1) {
			citations = append(citations, Citation{Case: match[1], File: file})
		}
	}

	cited := map[string]bool{}
	var report Report
	for _, c := range citations {
		cited[c.Case] = true
		if !defined[c.Case] {
			report.Unknown = append(report.Unknown, c)
		}
	}
	for id := range defined {
		if !cited[id] {
			report.Untested = append(report.Untested, id)
		}
	}
	sort.Slice(report.Untested, func(i, j int) bool { return lessCase(report.Untested[i], report.Untested[j]) })
	sort.Slice(report.Unknown, func(i, j int) bool {
		if report.Unknown[i].Case != report.Unknown[j].Case {
			return lessCase(report.Unknown[i].Case, report.Unknown[j].Case)
		}
		return report.Unknown[i].File < report.Unknown[j].File
	})
	return report, nil
}

// goTestCitations returns the case cited on the first line of the doc comment
// of every func TestXxx.
func goTestCitations(file, content string) ([]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var cases []string
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Doc == nil {
			continue
		}
		first := strings.TrimSpace(strings.TrimPrefix(fn.Doc.List[0].Text, "//"))
		if match := caseAtStart.FindStringSubmatch(first); match != nil {
			cases = append(cases, match[1])
		}
	}
	return cases, nil
}

func lessCase(a, b string) bool {
	sa, na := splitCase(a)
	sb, nb := splitCase(b)
	if sa != sb {
		return sa < sb
	}
	return na < nb
}

func splitCase(id string) (string, int) {
	spec, number, _ := strings.Cut(id, ".")
	n := 0
	for _, r := range number {
		n = n*10 + int(r-'0')
	}
	return spec, n
}
