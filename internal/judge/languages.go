package judge

import "sort"

// LanguageOption is the UI facing view of one judge language.
type LanguageOption struct {
	ID      string `json:"id"`
	Display string `json:"display"`
}

// LanguageOptions lists the languages this server advertises to the judge,
// skipping hidden and remote entries the same way Hydro filters them.
func LanguageOptions() []LanguageOption {
	languages := DefaultLanguages()
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
		display, _ := entry["display"].(string)
		if display == "" {
			display = key
		}
		options = append(options, LanguageOption{ID: key, Display: display})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options
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
