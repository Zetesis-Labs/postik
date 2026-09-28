package speccheck

import (
	"reflect"
	"testing"
)

const fixtureSpec = "# S99 · Fixture\n\n## S99.1 Primer caso\n\ntexto\n\n## S99.2 Segundo caso\n"

func fixtureGoTest(caseID string) string {
	return "package fixture\n\nimport \"testing\"\n\n" +
		"/" + "/ " + caseID + " descripción.\n" +
		"func TestFixture(t *testing.T) {}\n"
}

const fixtureE2E = "import { test } from '@playwright/test';\n\ntest('S99.3 algo en pantalla', async () => {});\n"

// S01.15 El anclaje detecta casos sin test y tests sin caso.
func TestCheckReportsUntestedAndUnknownCases(t *testing.T) {
	report, err := Check(Sources{
		Specs:   map[string]string{"specs/S99-fixture.md": fixtureSpec},
		GoTests: map[string]string{"fixture/fixture_test.go": fixtureGoTest("S99.1")},
		E2E:     map[string]string{"web/e2e/fixture.spec.ts": fixtureE2E},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.OK() {
		t.Fatal("the report must fail")
	}
	if !reflect.DeepEqual(report.Untested, []string{"S99.2"}) {
		t.Errorf("Untested = %v, want [S99.2]", report.Untested)
	}
	if len(report.Unknown) != 1 || report.Unknown[0].Case != "S99.3" || report.Unknown[0].File != "web/e2e/fixture.spec.ts" {
		t.Errorf("Unknown = %+v, want S99.3 in web/e2e/fixture.spec.ts", report.Unknown)
	}
}

func TestCheckPassesWhenEveryCaseIsCited(t *testing.T) {
	report, err := Check(Sources{
		Specs: map[string]string{"specs/S99-fixture.md": fixtureSpec},
		GoTests: map[string]string{
			"a_test.go": fixtureGoTest("S99.1"),
			"b_test.go": fixtureGoTest("S99.2"),
		},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.OK() {
		t.Fatalf("report = %+v", report)
	}
}

func TestOnlyTheFirstDocLineOfATestCounts(t *testing.T) {
	source := "package fixture\n\nimport \"testing\"\n\n" +
		"/" + "/ Un comentario cualquiera.\n" +
		"/" + "/ S99.1 esto no es la primera línea.\n" +
		"func TestFixture(t *testing.T) {}\n\n" +
		"/" + "/ S99.2 esto no es un test.\n" +
		"func helper() {}\n"
	report, err := Check(Sources{
		Specs:   map[string]string{"specs/S99-fixture.md": fixtureSpec},
		GoTests: map[string]string{"fixture_test.go": source},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !reflect.DeepEqual(report.Untested, []string{"S99.1", "S99.2"}) {
		t.Errorf("Untested = %v", report.Untested)
	}
}
