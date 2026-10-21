package problem

// FPS (Free Problem Set) import support, ported from Hydro's fps-importer
// addon (packages/fps-importer/index.ts).

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// MaxFPSImportSize mirrors the default `import-fps.limit` setting.
const MaxFPSImportSize = 64 << 20

// FPSProblem is one parsed <item> of an FPS problemset.
type FPSProblem struct {
	Title       string
	TimeLimit   string
	MemoryLimit string
	Description string
	Input       string
	Output      string
	Hint        string
	Sources     []string
	Samples     [][2]string
	Testdata    []FPSFile
	Solutions   []FPSSolution
	Images      []FPSImage
	Markdown    bool
}

// FPSFile is one file that should be written into the problem testdata.
type FPSFile struct {
	Name    string
	Content string
}

// FPSSolution is a solution shipped inside the problemset.
type FPSSolution struct {
	Language string
	Content  string
}

// FPSImage is an inline image carried as base64 in the problemset.
type FPSImage struct {
	Src    string
	Base64 string
}

type fpsText struct {
	Name     string `xml:"name,attr"`
	Filename string `xml:"filename,attr"`
	Value    string `xml:",innerxml"`
}

func (t fpsText) text() string {
	return stripCDATA(t.Value)
}

type fpsLimit struct {
	Unit  string `xml:"unit,attr"`
	Value string `xml:",chardata"`
}

type fpsSolution struct {
	Language string `xml:"language,attr"`
	Value    string `xml:",innerxml"`
}

type fpsImage struct {
	Src    string `xml:"src"`
	Base64 string `xml:"base64"`
}

type fpsItem struct {
	Title        []fpsText     `xml:"title"`
	TimeLimit    *fpsLimit     `xml:"time_limit"`
	MemoryLimit  *fpsLimit     `xml:"memory_limit"`
	Description  []fpsText     `xml:"description"`
	Input        []fpsText     `xml:"input"`
	Output       []fpsText     `xml:"output"`
	Hint         []fpsText     `xml:"hint"`
	Sources      []fpsText     `xml:"source"`
	SampleInput  []fpsText     `xml:"sample_input"`
	SampleOutput []fpsText     `xml:"sample_output"`
	TestInput    []fpsText     `xml:"test_input"`
	TestOutput   []fpsText     `xml:"test_output"`
	Solutions    []fpsSolution `xml:"solution"`
	Images       []fpsImage    `xml:"img"`
}

type fpsDocument struct {
	Items []fpsItem `xml:"item"`
}

func stripCDATA(value string) string {
	value = strings.ReplaceAll(value, "<![CDATA[", "")
	return strings.ReplaceAll(value, "]]>", "")
}

func firstText(items []fpsText) string {
	for _, item := range items {
		if value := strings.TrimSpace(item.text()); value != "" {
			return item.text()
		}
	}
	return ""
}

// ParseFPSXML parses one FPS XML document.
func ParseFPSXML(data []byte) ([]FPSProblem, error) {
	var doc fpsDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse fps xml: %w", err)
	}
	if len(doc.Items) == 0 {
		return nil, fmt.Errorf("no valid fps problem found")
	}
	result := make([]FPSProblem, 0, len(doc.Items))
	for _, item := range doc.Items {
		result = append(result, convertFPSItem(item))
	}
	return result, nil
}

// ParseFPS parses either one XML document or a zip archive of XML documents,
// the same input formats the upstream addon accepts.
func ParseFPS(data []byte) ([]FPSProblem, error) {
	if problems, err := ParseFPSXML(data); err == nil {
		return problems, nil
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("selected file is not a valid FPS problemset")
	}
	problems := make([]FPSProblem, 0)
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		file, err := entry.Open()
		if err != nil {
			continue
		}
		var buffer bytes.Buffer
		if _, err := buffer.ReadFrom(file); err != nil {
			file.Close()
			continue
		}
		file.Close()
		items, err := ParseFPSXML(buffer.Bytes())
		if err != nil {
			continue
		}
		problems = append(problems, items...)
	}
	if len(problems) == 0 {
		return nil, fmt.Errorf("no valid fps format file found")
	}
	return problems, nil
}

func convertFPSItem(item fpsItem) FPSProblem {
	titles := make([]string, 0, len(item.Title))
	for _, title := range item.Title {
		if value := strings.TrimSpace(title.text()); value != "" {
			titles = append(titles, value)
		}
	}
	result := FPSProblem{
		Title:       strings.Join(titles, " "),
		Description: firstText(item.Description),
		Input:       firstText(item.Input),
		Output:      firstText(item.Output),
		Hint:        firstText(item.Hint),
	}
	if item.TimeLimit != nil {
		result.TimeLimit = strings.TrimSpace(item.TimeLimit.Value) + item.TimeLimit.Unit
	}
	if item.MemoryLimit != nil {
		result.MemoryLimit = strings.TrimSpace(item.MemoryLimit.Value) + item.MemoryLimit.Unit
	}
	for _, source := range item.Sources {
		if value := strings.TrimSpace(source.text()); value != "" {
			result.Sources = append(result.Sources, value)
		}
	}
	for index, input := range item.SampleInput {
		output := ""
		if index < len(item.SampleOutput) {
			output = item.SampleOutput[index].text()
		}
		result.Samples = append(result.Samples, [2]string{input.text(), output})
	}
	result.Testdata = fpsTestdata(item)
	for _, solution := range item.Solutions {
		result.Solutions = append(result.Solutions, FPSSolution{
			Language: solution.Language,
			Content:  stripCDATA(solution.Value),
		})
	}
	for _, image := range item.Images {
		result.Images = append(result.Images, FPSImage{
			Src:    strings.TrimSpace(image.Src),
			Base64: strings.TrimSpace(image.Base64),
		})
	}
	for _, value := range []string{result.Description, result.Input, result.Output, result.Hint} {
		if strings.Contains(value, "[md]") {
			result.Markdown = true
			break
		}
	}
	return result
}

// fpsTestdata names the testdata files the way the upstream addTestdata helper
// does: the `filename` attribute wins, then `name`, then `<index>.<ext>`.
func fpsTestdata(item fpsItem) []FPSFile {
	files := make([]FPSFile, 0, len(item.TestInput)+len(item.TestOutput)+1)
	add := func(node fpsText, index, ext string) {
		name := strings.TrimSpace(node.Name)
		if name == "" {
			name = index
		}
		if filename := strings.TrimSpace(node.Filename); filename != "" {
			name = filename
		} else {
			name += "." + ext
		}
		files = append(files, FPSFile{Name: path.Base(name), Content: node.text()})
	}
	switch {
	case len(item.TestOutput) > 0:
		for index := range item.TestInput {
			add(item.TestInput[index], fmt.Sprintf("%d", index+1), "in")
			if index < len(item.TestOutput) {
				add(item.TestOutput[index], fmt.Sprintf("%d", index+1), "out")
			}
		}
	case len(item.TestInput) >= 2:
		// Some HUSTOJ exports only carry a flat test_input list of in/out pairs.
		for index := 0; index < len(item.TestInput)/2; index++ {
			add(item.TestInput[2*index], fmt.Sprintf("%d", index+1), "in")
			add(item.TestInput[2*index+1], fmt.Sprintf("%d", index+1), "out")
		}
	}
	return files
}

// Content renders the statement as HTML with the same section layout and
// sample blocks as buildContent(..., 'html') upstream.
func (p FPSProblem) Content() string {
	parts := make([]string, 0, 8)
	line := func(title, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		parts = append(parts, fmt.Sprintf("<h2>%s</h2>\n%s", title, value))
	}
	line("Problem Description", p.Description)
	line("Input Format", p.Input)
	line("Output Format", p.Output)
	if len(p.Samples) > 0 {
		var samples strings.Builder
		for index, sample := range p.Samples {
			fmt.Fprintf(&samples,
				`<pre><code class="language-input%d">%s</code></pre><pre><code class="language-output%d">%s</code></pre>`,
				index+1, sample[0], index+1, sample[1])
		}
		parts = append(parts, samples.String())
	}
	line("Hint", p.Hint)
	line("Source", strings.Join(p.Sources, " "))
	content := strings.Join(parts, "\n")
	content = strings.ReplaceAll(content, "<math xm<x>lns=", "<math xmlns=")
	content = strings.ReplaceAll(content, "[md]", "")
	content = strings.ReplaceAll(content, "[/md]", "")
	return content
}

// Config is the YAML written into testdata/config.yaml.
func (p FPSProblem) Config() string {
	lines := make([]string, 0, 2)
	if p.TimeLimit != "" {
		lines = append(lines, "time: "+p.TimeLimit)
	}
	if p.MemoryLimit != "" {
		lines = append(lines, "memory: "+p.MemoryLimit)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// Tags splits the <source> fields on whitespace, like the upstream importer.
func (p FPSProblem) Tags() []string {
	result := make([]string, 0, len(p.Sources))
	seen := map[string]bool{}
	for _, source := range p.Sources {
		for _, tag := range strings.Fields(source) {
			if !seen[tag] {
				seen[tag] = true
				result = append(result, tag)
			}
		}
	}
	return result
}

// ImageName picks a random name with the image's original extension.
func ImageName(src string) string {
	extension := path.Ext(src)
	if len(extension) > 16 || strings.ContainsAny(extension, "/\\") {
		extension = ""
	}
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return "image" + extension
	}
	return hex.EncodeToString(buffer) + extension
}
