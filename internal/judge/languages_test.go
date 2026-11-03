package judge

import "testing"

func TestParseLanguagesInheritanceAndFiltering(t *testing.T) {
	languages, err := ParseLanguages(`
__hint__:
  - ignored
gcc:
  display: GNU C
  compile: gcc -o foo foo.c
  code_file: foo.c
gcc.debug:
  display: GNU C Debug
hidden:
  display: Hidden
  hidden: true
remote:
  display: Remote
  remote: true
disabled:
  display: Disabled
  disabled: true
`)
	if err != nil {
		t.Fatalf("ParseLanguages: %v", err)
	}
	debug, ok := languages["gcc.debug"].(map[string]any)
	if !ok {
		t.Fatalf("gcc.debug missing: %#v", languages["gcc.debug"])
	}
	if debug["compile"] != "gcc -o foo foo.c" || debug["code_file"] != "foo.c" {
		t.Fatalf("inherited fields = %#v", debug)
	}
	if debug["key"] != "gcc.debug" || debug["display"] != "GNU C Debug" {
		t.Fatalf("normalized fields = %#v", debug)
	}
	options := LanguageOptionsFor(languages)
	visible := map[string]bool{}
	for _, option := range options {
		visible[option.ID] = true
	}
	if !visible["gcc"] || !visible["gcc.debug"] || visible["hidden"] || visible["remote"] || visible["disabled"] {
		t.Fatalf("visible languages = %#v", visible)
	}
}

func TestParseLanguagesRejectsInvalidConfig(t *testing.T) {
	for _, input := range []string{"", "invalid: [", "scalar", "_hint:\n  - only\n"} {
		if _, err := ParseLanguages(input); err == nil {
			t.Fatalf("ParseLanguages accepted %q", input)
		}
	}
}

func TestDefaultLanguagesYAMLIsParseable(t *testing.T) {
	languages, err := ParseLanguages(DefaultLanguagesYAML())
	if err != nil {
		t.Fatalf("parse default YAML: %v", err)
	}
	if len(languages) != len(DefaultLanguages()) {
		t.Fatalf("default language count = %d, want %d", len(languages), len(DefaultLanguages()))
	}
}
