package judge

func DefaultLanguages() map[string]any {
	return map[string]any{
		"cpp": map[string]any{
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
		},
		"python3": map[string]any{
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
		},
	}
}
