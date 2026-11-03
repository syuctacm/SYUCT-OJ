package judge

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LanguageOption is the UI facing view of one judge language.
type LanguageOption struct {
	ID      string `json:"id"`
	Display string `json:"display"`
}

// LanguageOptions lists the languages this server advertises to the judge,
// skipping hidden and remote entries the same way Hydro filters them.
func LanguageOptions() []LanguageOption {
	return LanguageOptionsFor(DefaultLanguages())
}

func LanguageOptionsFor(languages map[string]any) []LanguageOption {
	options := make([]LanguageOption, 0, len(languages))
	for key, raw := range languages {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if hidden, _ := entry["hidden"].(bool); hidden {
			continue
		}
		if remote, _ := entry["remote"].(bool); remote {
			continue
		}
		if disabled, _ := entry["disabled"].(bool); disabled {
			continue
		}
		display, _ := entry["display"].(string)
		if display == "" {
			display = key
		}
		options = append(options, LanguageOption{ID: key, Display: display})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options
}

// ParseLanguages loads Hydro's hydrooj.langs YAML format. Dotted keys inherit
// options from their base entry, matching packages/common/lang.ts.
func ParseLanguages(input string) (map[string]any, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("language config is required")
	}
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(input), &raw); err != nil {
		return nil, fmt.Errorf("invalid language config: %w", err)
	}
	languages := make(map[string]any)
	for key, value := range raw {
		if strings.HasPrefix(key, "_") {
			continue
		}
		entry, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("language %s must be an object", key)
		}
		languages[key] = cloneLanguageMap(entry)
	}
	if len(languages) == 0 {
		return nil, fmt.Errorf("language config is empty")
	}
	for key, value := range languages {
		entry := value.(map[string]any)
		if dot := strings.IndexByte(key, '.'); dot > 0 {
			if parent, ok := languages[key[:dot]].(map[string]any); ok {
				for parentKey, parentValue := range parent {
					if _, exists := entry[parentKey]; !exists {
						entry[parentKey] = parentValue
					}
				}
			}
		}
		normalizeLanguage(key, entry)
	}
	return languages, nil
}

func cloneLanguageMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func normalizeLanguage(key string, entry map[string]any) {
	if _, ok := entry["display"]; !ok {
		entry["display"] = key
	}
	if entry["display"] == "" {
		entry["display"] = key
	}
	if _, ok := entry["code_file"]; !ok || entry["code_file"] == "" {
		entry["code_file"] = "foo." + key
	}
	if _, ok := entry["execute"]; !ok || entry["execute"] == "" {
		entry["execute"] = "/w/foo"
	}
	if _, ok := entry["target"]; !ok || entry["target"] == "" {
		entry["target"] = "foo"
	}
	if _, ok := entry["highlight"]; !ok || entry["highlight"] == "" {
		entry["highlight"] = key
	}
	if _, ok := entry["monaco"]; !ok || entry["monaco"] == "" {
		entry["monaco"] = entry["highlight"]
	}
	if _, ok := entry["time_limit_rate"]; !ok || entry["time_limit_rate"] == 0 {
		entry["time_limit_rate"] = 1
	}
	if _, ok := entry["memory_limit_rate"]; !ok || entry["memory_limit_rate"] == 0 {
		entry["memory_limit_rate"] = 1
	}
	entry["key"] = key
	for _, field := range []string{"hidden", "remote", "disabled", "isBinary"} {
		entry[field] = entry[field] == true
	}
	if _, ok := entry["validAs"]; !ok {
		entry["validAs"] = map[string]any{}
	}
}

// DefaultLanguagesYAML returns the minimal built-in registry in the same YAML
// shape accepted by Hydro's hydrooj.langs setting.
func DefaultLanguagesYAML() string {
	data, _ := yaml.Marshal(DefaultLanguages())
	return string(data)
}

func DefaultLanguages() map[string]any {
	cpp := map[string]any{
		"key":               "cpp",
		"display":           "C++17",
		"target":            "foo",
		"compile":           "/usr/bin/g++ -O2 -std=c++17 -o foo foo.cpp",
		"execute":           "/w/foo",
		"code_file":         "foo.cpp",
		"highlight":         "cpp",
		"monaco":            "cpp",
		"time_limit_rate":   1,
		"memory_limit_rate": 1,
		"hidden":            false,
		"remote":            false,
		"validAs":           map[string]string{},
	}
	python := map[string]any{
		"key":               "python3",
		"display":           "Python 3",
		"target":            "foo",
		"compile":           `/usr/bin/python3 -c "import py_compile; py_compile.compile('/w/foo.py', '/w/foo', doraise=True)"`,
		"execute":           "/usr/bin/python3 foo",
		"code_file":         "foo.py",
		"highlight":         "python",
		"monaco":            "python",
		"time_limit_rate":   2,
		"memory_limit_rate": 2,
		"hidden":            false,
		"remote":            false,
		"validAs":           map[string]string{},
	}
	return map[string]any{
		"cpp":     cpp,
		"cc":      hiddenLanguageAlias(cpp, "cc", "foo.cc", "/usr/bin/g++ -O2 -std=c++17 -o foo foo.cc"),
		"python3": python,
		"py":      hiddenLanguageAlias(python, "py", "foo.py", ""),
	}
}

// hydrojudge resolves checker/interactor languages from their filename
// extension. Keep extension aliases hidden from the submission language list,
// but broadcast them so files such as chk.cc and interactor.py can compile.
func hiddenLanguageAlias(base map[string]any, key, codeFile, compileCommand string) map[string]any {
	alias := make(map[string]any, len(base))
	for name, value := range base {
		alias[name] = value
	}
	alias["key"] = key
	alias["code_file"] = codeFile
	if compileCommand != "" {
		alias["compile"] = compileCommand
	}
	alias["hidden"] = true
	return alias
}
