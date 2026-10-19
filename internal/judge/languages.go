package judge

func DefaultLanguages() map[string]any {
	return map[string]any{
		"cpp": map[string]any{
			"display":           "C++17",
			"compile":           "g++ -O2 -std=c++17 -o /w/main /w/main.cpp",
			"execute":           "/w/main",
			"code_file":         "main.cpp",
			"highlight":         "cpp",
			"monaco":            "cpp",
			"time_limit_rate":   1,
			"memory_limit_rate": 1,
			"hidden":            false,
			"remote":            false,
			"validAs":           map[string]string{},
		},
		"python3": map[string]any{
			"display":           "Python 3",
			"compile":           "python3 -m py_compile /w/main.py",
			"execute":           "python3 /w/main.py",
			"code_file":         "main.py",
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
