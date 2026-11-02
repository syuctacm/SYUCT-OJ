package handler

import "testing"

func TestValidAssetURL(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"/nav_logo_dark.png", true},
		{"https://example.com/avatar/", true},
		{"http://example.com/logo.png", true},
		{"//cn.gravatar.com/avatar/", true},
		{"", false},
		{"//", false},
		{"javascript:alert(1)", false},
		{"data:text/html,bad", false},
		{"https://", false},
		{"relative/logo.png", false},
		{"/logo\nnext", false},
	}
	for _, test := range tests {
		if got := validAssetURL(test.value); got != test.valid {
			t.Errorf("validAssetURL(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestValidHomepageYAML(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"- width: 12\n  bulletin: true\n  contest: 3\n", true},
		{"- {width: 8, discussion: 20}\n- {width: 4, suggestion: true}\n", true},
		{"", false},
		{"not: [valid", false},
		{"[]", false},
		{"- contest: 1", false},
		{"- width: 0\n  contest: 1", false},
		{"- width: 13\n  contest: 1", false},
		{"- width: 12\n  contest: {limit: 1}", false},
	}
	for _, test := range tests {
		if got := validHomepageYAML(test.value); got != test.valid {
			t.Errorf("validHomepageYAML(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}
