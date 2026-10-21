package problem

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

const sampleFPS = `<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2" url="https://github.com/zhblue/freeproblemset/">
  <generator name="HUSTOJ"/>
  <item>
    <title><![CDATA[A+B Problem]]></title>
    <time_limit unit="s"><![CDATA[1]]></time_limit>
    <memory_limit unit="mb"><![CDATA[128]]></memory_limit>
    <description><![CDATA[<p>Calculate <b>a+b</b>.</p>]]></description>
    <input><![CDATA[Two integers.]]></input>
    <output><![CDATA[Their sum.]]></output>
    <sample_input><![CDATA[1 2]]></sample_input>
    <sample_output><![CDATA[3]]></sample_output>
    <test_input name="1"><![CDATA[1 2]]></test_input>
    <test_output><![CDATA[3]]></test_output>
    <test_input filename="big.in"><![CDATA[100000 200000]]></test_input>
    <test_output filename="big.out"><![CDATA[300000]]></test_output>
    <hint><![CDATA[Use int64.]]></hint>
    <source>基本 入门</source>
    <solution language="C++"><![CDATA[#include <iostream>]]></solution>
    <img><src>https://example.com/pic.png</src><base64>aGVsbG8=</base64></img>
  </item>
</fps>`

func TestParseFPSXML(t *testing.T) {
	items, err := ParseFPS([]byte(sampleFPS))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	item := items[0]
	if item.Title != "A+B Problem" {
		t.Fatalf("title = %q", item.Title)
	}
	if item.TimeLimit != "1s" || item.MemoryLimit != "128mb" {
		t.Fatalf("limits = %q %q", item.TimeLimit, item.MemoryLimit)
	}
	if item.Description != "<p>Calculate <b>a+b</b>.</p>" {
		t.Fatalf("description = %q", item.Description)
	}
	if len(item.Samples) != 1 || item.Samples[0][0] != "1 2" || item.Samples[0][1] != "3" {
		t.Fatalf("samples = %#v", item.Samples)
	}
	if item.Markdown {
		t.Fatal("html problemset marked as markdown")
	}
	if tags := item.Tags(); len(tags) != 2 || tags[0] != "基本" || tags[1] != "入门" {
		t.Fatalf("tags = %#v", tags)
	}
	if len(item.Solutions) != 1 || item.Solutions[0].Language != "C++" {
		t.Fatalf("solutions = %#v", item.Solutions)
	}
	if len(item.Images) != 1 || item.Images[0].Src != "https://example.com/pic.png" {
		t.Fatalf("images = %#v", item.Images)
	}
	if config := item.Config(); config != "time: 1s\nmemory: 128mb\n" {
		t.Fatalf("config = %q", config)
	}
	content := item.Content()
	for _, want := range []string{
		"<h2>Problem Description</h2>",
		"<h2>Input Format</h2>",
		`<pre><code class="language-input1">1 2</code></pre>`,
		`<pre><code class="language-output1">3</code></pre>`,
		"<h2>Source</h2>\n基本 入门",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content misses %q:\n%s", want, content)
		}
	}
}

func TestParseFPSTestdataNames(t *testing.T) {
	items, err := ParseFPS([]byte(sampleFPS))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range items[0].Testdata {
		files[file.Name] = file.Content
	}
	want := map[string]string{
		"1.in":    "1 2",
		"1.out":   "3",
		"big.in":  "100000 200000",
		"big.out": "300000",
	}
	if len(files) != len(want) {
		t.Fatalf("files = %#v", files)
	}
	for name, content := range want {
		if files[name] != content {
			t.Fatalf("file %s = %q", name, files[name])
		}
	}
}

func TestParseFPSMarkdownAndFlatTestInput(t *testing.T) {
	source := `<fps><item>
	  <title>Markdown problem</title>
	  <description><![CDATA[[md]**bold**[/md]]]></description>
	  <test_input><![CDATA[1 1]]></test_input>
	  <test_input><![CDATA[2]]></test_input>
	</item></fps>`
	items, err := ParseFPS([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	item := items[0]
	if !item.Markdown {
		t.Fatal("markdown problem not detected")
	}
	if strings.Contains(item.Content(), "[md]") {
		t.Fatalf("markdown markers left in content: %q", item.Content())
	}
	files := map[string]string{}
	for _, file := range item.Testdata {
		files[file.Name] = file.Content
	}
	if files["1.in"] != "1 1" || files["1.out"] != "2" {
		t.Fatalf("flat test_input files = %#v", files)
	}
	if item.Config() != "" {
		t.Fatalf("config without limits = %q", item.Config())
	}
}

func TestParseFPSZip(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("a.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(sampleFPS)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	items, err := ParseFPS(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "A+B Problem" {
		t.Fatalf("zip items = %#v", items)
	}
}

func TestParseFPSRejectsGarbage(t *testing.T) {
	if _, err := ParseFPS([]byte("not a problemset")); err == nil {
		t.Fatal("garbage accepted")
	}
	if _, err := ParseFPS([]byte("<fps></fps>")); err == nil {
		t.Fatal("empty problemset accepted")
	}
}

func TestImageName(t *testing.T) {
	name := ImageName("https://example.com/a/b/pic.png")
	if !strings.HasSuffix(name, ".png") || len(name) != 12 {
		t.Fatalf("image name = %q", name)
	}
}
