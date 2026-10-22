package problem

// Hydro / ICPC zip problem-package import, ported from ProblemModel.import in
// packages/hydrooj/src/model/problem.ts. The FPS importer lives in fps.go; this
// file handles the native zip layout:
//
//	problem.yaml          + problem_<lang>.md|pdf + testdata/ + additional_file/ ...
//	<dir>/problem.yaml    (one package per directory, as produced by problem export)
//
// ICPC problem-package directories (statement/, data/, attachments/, include/,
// output_validators/) are mapped onto testdata/additional files like upstream.

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// MaxHydroImportSize caps the uncompressed package size.
const MaxHydroImportSize = 256 << 20

// HydroPackage is one parsed problem package.
type HydroPackage struct {
	Dir        string
	PID        string
	Title      string
	Content    string
	Tags       []string
	Difficulty int
	Hidden     bool
	Owner      int64
	Config     string
	Testdata   []FPSFile
	Additional []FPSFile
	Solutions  []string
}

type hydroProblemYAML struct {
	PID        any    `yaml:"pid"`
	Title      string `yaml:"title"`
	Name       string `yaml:"name"`
	Tag        []any  `yaml:"tag"`
	Difficulty any    `yaml:"difficulty"`
	Hidden     bool   `yaml:"hidden"`
	Owner      int64  `yaml:"owner"`
	Content    string `yaml:"content"`
	Limits     *struct {
		TimeLimit any `yaml:"time_limit"`
		Memory    any `yaml:"memory"`
	} `yaml:"limits"`
}

// ParseHydroPackages reads one zip archive and returns every problem package it
// contains.
func ParseHydroPackages(data []byte) ([]HydroPackage, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("selected file is not a valid zip archive")
	}
	files := map[string][]byte{}
	var total int64
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(entry.Name)
		if strings.HasPrefix(name, "../") || strings.Contains(name, "__MACOSX") {
			continue
		}
		total += int64(entry.UncompressedSize64)
		if total > MaxHydroImportSize {
			return nil, fmt.Errorf("problemset is too large")
		}
		file, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("unable to read %s", name)
		}
		content, err := io.ReadAll(io.LimitReader(file, MaxHydroImportSize))
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("unable to read %s", name)
		}
		files[name] = content
	}
	roots := map[string]bool{}
	for name := range files {
		if path.Base(name) != "problem.yaml" {
			continue
		}
		dir := path.Dir(name)
		if dir == "." {
			roots[""] = true
		} else {
			roots[dir] = true
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no problem.yaml found in the archive")
	}
	ordered := make([]string, 0, len(roots))
	for root := range roots {
		ordered = append(ordered, root)
	}
	sort.Strings(ordered)
	packages := make([]HydroPackage, 0, len(ordered))
	for _, root := range ordered {
		pkg, err := buildHydroPackage(files, root)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

func hydroPrefix(root string) string {
	if root == "" {
		return ""
	}
	return root + "/"
}

func buildHydroPackage(files map[string][]byte, root string) (HydroPackage, error) {
	prefix := hydroPrefix(root)
	label := root
	if label == "" {
		label = "(root)"
	}
	var meta hydroProblemYAML
	if err := yaml.Unmarshal(files[prefix+"problem.yaml"], &meta); err != nil {
		return HydroPackage{}, fmt.Errorf("%s: invalid problem.yaml: %w", label, err)
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = strings.TrimSpace(meta.Name)
	}
	if title == "" {
		return HydroPackage{}, fmt.Errorf("%s: problem.yaml has no title", label)
	}
	pkg := HydroPackage{
		Dir:        root,
		PID:        scalarString(meta.PID),
		Title:      title,
		Content:    hydroContent(files, prefix, meta.Content),
		Hidden:     meta.Hidden,
		Owner:      meta.Owner,
		Difficulty: scalarInt(meta.Difficulty),
	}
	for _, tag := range meta.Tag {
		if value := strings.TrimSpace(scalarString(tag)); value != "" {
			pkg.Tags = append(pkg.Tags, value)
		}
	}
	config := map[string]any{}
	if raw, ok := files[prefix+"testdata/config.yaml"]; ok {
		_ = yaml.Unmarshal(raw, &config)
	}
	if raw, ok := files[prefix+"domjudge-problem.ini"]; ok {
		if value, found := iniValue(raw, "timelimit"); found {
			if seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				config["time"] = fmt.Sprintf("%.0fms", seconds*1000)
			}
		}
		if value, found := iniValue(raw, "externalid"); found && strings.TrimSpace(value) != "" {
			pkg.PID = strings.TrimSpace(value)
		}
	}
	if meta.Limits != nil {
		if value := scalarString(meta.Limits.TimeLimit); value != "" {
			if seconds, err := strconv.ParseFloat(value, 64); err == nil {
				config["time"] = fmt.Sprintf("%.0fms", seconds*1000)
			}
		}
		if value := scalarString(meta.Limits.Memory); value != "" {
			if memory, err := strconv.ParseFloat(value, 64); err == nil {
				config["memory"] = fmt.Sprintf("%.0fm", memory)
			}
		}
	}

	extraFiles := map[string]bool{}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		if rest == "" || rest == "problem.yaml" {
			continue
		}
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) < 2 {
			continue
		}
		dir, sub := parts[0], parts[1]
		base := path.Base(sub)
		content := files[name]
		switch dir {
		case "testdata":
			pkg.Testdata = append(pkg.Testdata, FPSFile{Name: base, Content: string(content)})
		case "attachments":
			// Upstream stores attachments as testdata *and* additional files,
			// and exposes them to the judge.
			pkg.Testdata = append(pkg.Testdata, FPSFile{Name: base, Content: string(content)})
			pkg.Additional = append(pkg.Additional, FPSFile{Name: base, Content: string(content)})
			extraFiles[base] = true
		case "generators", "include":
			pkg.Testdata = append(pkg.Testdata, FPSFile{Name: base, Content: string(content)})
			extraFiles[base] = true
		case "data":
			pkg.Testdata = append(pkg.Testdata, FPSFile{Name: base, Content: string(content)})
			if strings.HasPrefix(sub, "sample/") {
				pkg.Additional = append(pkg.Additional, FPSFile{Name: base, Content: string(content)})
			}
		case "output_validators":
			if base == "testlib.h" {
				continue
			}
			pkg.Testdata = append(pkg.Testdata, FPSFile{Name: base, Content: string(content)})
			if strings.HasPrefix(sub, "checker/") {
				config["checker_type"] = "testlib"
				config["checker"] = base
			} else if strings.HasPrefix(sub, "interactor/") {
				config["type"] = "interactive"
				config["interactor"] = base
			}
		case "additional_file", "statement", "problem_statement":
			pkg.Additional = append(pkg.Additional, FPSFile{Name: base, Content: string(content)})
		case "solution":
			pkg.Solutions = append(pkg.Solutions, string(content))
		}
	}
	if len(extraFiles) > 0 {
		keys := make([]string, 0, len(extraFiles))
		for name := range extraFiles {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		values := make([]any, 0, len(keys))
		for _, name := range keys {
			values = append(values, name)
		}
		config["user_extra_files"] = values
		config["judge_extra_files"] = values
	}
	if len(config) > 0 {
		encoded, err := yaml.Marshal(config)
		if err != nil {
			return HydroPackage{}, fmt.Errorf("%s: unable to build the judge config", label)
		}
		pkg.Config = string(encoded)
	}
	if pkg.Config != "" {
		// Keep testdata/config.yaml in sync with what the importer derived.
		filtered := make([]FPSFile, 0, len(pkg.Testdata)+1)
		for _, file := range pkg.Testdata {
			if strings.EqualFold(path.Base(file.Name), "config.yaml") {
				continue
			}
			filtered = append(filtered, file)
		}
		pkg.Testdata = append([]FPSFile{{Name: "config.yaml", Content: pkg.Config}}, filtered...)
	}
	return pkg, nil
}

// hydroContent resolves the statement, mirroring findOverrideContent.
func hydroContent(files map[string][]byte, prefix, fallback string) string {
	if raw, ok := files[prefix+"problem.md"]; ok {
		return string(raw)
	}
	if _, ok := files[prefix+"problem.pdf"]; ok {
		return "@[PDF](file://problem.pdf)"
	}
	best := ""
	bestLang := ""
	for name, raw := range files {
		rest := strings.TrimPrefix(name, prefix)
		if strings.Contains(rest, "/") || !strings.HasPrefix(rest, "problem_") || !strings.HasSuffix(rest, ".md") {
			continue
		}
		lang := strings.TrimSuffix(strings.TrimPrefix(rest, "problem_"), ".md")
		if lang == "zh" {
			return string(raw)
		}
		if best == "" || lang < bestLang {
			best, bestLang = string(raw), lang
		}
	}
	if best != "" {
		return best
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "No content"
}

func iniValue(raw []byte, key string) (string, bool) {
	for _, line := range strings.Split(string(raw), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), key) {
			return strings.TrimSpace(parts[1]), true
		}
	}
	return "", false
}

func scalarString(value any) string {
	switch current := value.(type) {
	case nil:
		return ""
	case string:
		return current
	case int:
		return strconv.Itoa(current)
	case int64:
		return strconv.FormatInt(current, 10)
	case float64:
		return strconv.FormatFloat(current, 'f', -1, 64)
	default:
		return fmt.Sprint(current)
	}
}

func scalarInt(value any) int {
	switch current := value.(type) {
	case int:
		return current
	case int64:
		return int(current)
	case float64:
		return int(current)
	default:
		parsed, _ := strconv.Atoi(scalarString(value))
		return parsed
	}
}
