package problem

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestParseHydroPackage(t *testing.T) {
	data := buildZip(t, map[string]string{
		"mypkg/problem.yaml": `pid: ABC1001
title: 测试题目
tag: [基础, 模拟]
difficulty: 3
limits:
  time_limit: 1.5
  memory: 128
`,
		"mypkg/problem_zh.md":             "# 题面\n\n给定 a 和 b。\n",
		"mypkg/testdata/1.in":             "1 2\n",
		"mypkg/testdata/1.out":            "3\n",
		"mypkg/testdata/config.yaml":      "checker_type: default\n",
		"mypkg/domjudge-problem.ini":      "timelimit=2\nexternalid=abc1001\n",
		"mypkg/additional_file/notes.txt": "hello\n",
		"mypkg/solution/sol.cpp":          "int main() { return 0; }\n",
		"__MACOSX/._problem.yaml":         "junk",
	})
	packages, err := ParseHydroPackages(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 {
		t.Fatalf("packages = %d", len(packages))
	}
	pkg := packages[0]
	if pkg.Dir != "mypkg" || pkg.PID != "abc1001" || pkg.Title != "测试题目" {
		t.Fatalf("package = %+v", pkg)
	}
	if strings.Join(pkg.Tags, ",") != "基础,模拟" || pkg.Difficulty != 3 {
		t.Fatalf("tags/difficulty = %v %d", pkg.Tags, pkg.Difficulty)
	}
	if !strings.Contains(pkg.Content, "给定 a 和 b") {
		t.Fatalf("content = %q", pkg.Content)
	}
	// domjudge-problem.ini sets 2s, problem.yaml limits override it with 1.5s.
	if !strings.Contains(pkg.Config, "time: 1500ms") || !strings.Contains(pkg.Config, "memory: 128m") {
		t.Fatalf("config = %q", pkg.Config)
	}
	files := map[string]string{}
	for _, file := range pkg.Testdata {
		files[file.Name] = file.Content
	}
	if files["1.in"] != "1 2\n" || files["1.out"] != "3\n" || files["config.yaml"] != pkg.Config {
		t.Fatalf("testdata = %#v", files)
	}
	if len(pkg.Additional) != 1 || pkg.Additional[0].Name != "notes.txt" {
		t.Fatalf("additional = %#v", pkg.Additional)
	}
	if len(pkg.Solutions) != 1 || !strings.Contains(pkg.Solutions[0], "int main") {
		t.Fatalf("solutions = %#v", pkg.Solutions)
	}
}

func TestParseHydroRootPackageAndICPC(t *testing.T) {
	data := buildZip(t, map[string]string{
		"problem.yaml":                     "title: ICPC 题\n",
		"problem.md":                       "# ICPC 题面\n",
		"data/sample/1.in":                 "1\n",
		"data/secret/1.in":                 "2\n",
		"attachments/statement.pdf":        "pdf",
		"output_validators/checker/chk.cc": "// checker",
	})
	packages, err := ParseHydroPackages(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Dir != "" {
		t.Fatalf("packages = %#v", packages)
	}
	pkg := packages[0]
	if pkg.Title != "ICPC 题" || pkg.Content != "# ICPC 题面\n" {
		t.Fatalf("package = %+v", pkg)
	}
	if !strings.Contains(pkg.Config, "checker_type: testlib") || !strings.Contains(pkg.Config, "checker: chk.cc") {
		t.Fatalf("config = %q", pkg.Config)
	}
	names := map[string]bool{}
	for _, file := range pkg.Testdata {
		names[file.Name] = true
	}
	if !names["chk.cc"] || !names["1.in"] {
		t.Fatalf("testdata = %#v", names)
	}
	// statement.pdf is an additional file, and sample data is duplicated there.
	additional := map[string]bool{}
	for _, file := range pkg.Additional {
		additional[file.Name] = true
	}
	if !additional["statement.pdf"] || !additional["1.in"] {
		t.Fatalf("additional = %#v", additional)
	}
}

func TestParseHydroMultiplePackages(t *testing.T) {
	data := buildZip(t, map[string]string{
		"1001/problem.yaml": "title: 第一题\n",
		"1001/problem.md":   "第一题面\n",
		"1002/problem.yaml": "title: 第二题\n",
		"1002/problem.md":   "第二题面\n",
	})
	packages, err := ParseHydroPackages(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].Title != "第一题" || packages[1].Title != "第二题" {
		t.Fatalf("packages = %#v", packages)
	}
}

func TestParseHydroRejectsBadArchives(t *testing.T) {
	if _, err := ParseHydroPackages([]byte("not a zip")); err == nil {
		t.Fatal("garbage accepted")
	}
	if _, err := ParseHydroPackages(buildZip(t, map[string]string{"readme.txt": "hi"})); err == nil {
		t.Fatal("archive without problem.yaml accepted")
	}
	if _, err := ParseHydroPackages(buildZip(t, map[string]string{"problem.yaml": "title: ''"})); err == nil {
		t.Fatal("package without title accepted")
	}
}
