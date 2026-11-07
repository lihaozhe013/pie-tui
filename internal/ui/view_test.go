package ui

import (
	"path/filepath"
	"testing"
)

func TestDisplayPathCollapsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"home itself", home, "~"},
		{"under home", filepath.Join(home, "dev", "pie-tui"), "~/dev/pie-tui"},
		{"unclean under home", home + "//dev//pie-tui/", "~/dev/pie-tui"},
		{"outside home", "/var/data/project", "/var/data/project"},
		{"prefix must respect boundaries", home + "-extra/dev", home + "-extra/dev"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := displayPath(tc.in); got != tc.want {
				t.Fatalf("displayPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestShortenPathElidesDeepPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if got := shortenPath(""); got != "unknown directory" {
		t.Fatalf("shortenPath(\"\") = %q", got)
	}
	// The leading root element counts, so three visible segments stay put.
	if got := shortenPath("/a/b/c"); got != "/a/b/c" {
		t.Fatalf("shortenPath = %q", got)
	}
	if got := shortenPath("/a/b/c/d"); got != "…/b/c/d" {
		t.Fatalf("shortenPath deep = %q", got)
	}
	if got := shortenPath(filepath.Join(home, "dev", "pie-tui")); got != "~/dev/pie-tui" {
		t.Fatalf("shortenPath home = %q", got)
	}
}
