package bookpath

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	for _, tc := range []struct{ href, want string }{
		{"../Text/中文%20章.xhtml?x=1#note", "书/Text/中文 章.xhtml"},
		{"../Text/100%25.xhtml", "书/Text/100%.xhtml"},
		{"../Text/%252e%252e.xhtml", "书/Text/%2e%2e.xhtml"},
		{"-chapter.xhtml", "书/Deep/-chapter.xhtml"},
	} {
		got, e := Resolve("书/Deep/pkg.opf", Href(tc.href))
		if e != nil || string(got) != tc.want {
			t.Fatalf("%q: %q %v", tc.href, got, e)
		}
	}
	for _, h := range []string{"../../../escape", "%2fetc/passwd", "https://example.com/x", "//host/x", "../%5cx", "../C%3a/x", "%xx", ""} {
		if p, e := Resolve("书/Deep/pkg.opf", Href(h)); e == nil {
			t.Fatalf("accepted %q as %q", h, p)
		}
	}
}

func TestResolveReferenceComponents(t *testing.T) {
	for _, tc := range []struct {
		href string
		want Reference
	}{
		{"#%E6%B3%A8%201", Reference{Path: "书/Text/ch.xhtml", Fragment: "注 1"}},
		{"?edition=2#note", Reference{Path: "书/Text/ch.xhtml", Fragment: "note", Query: "edition=2"}},
		{"", Reference{Path: "书/Text/ch.xhtml"}},
		{"../Images/a%23b%25.svg?x=%23#%2520", Reference{Path: "书/Images/a#b%.svg", Fragment: "%20", Query: "x=%23"}},
		{"https://example.invalid/x#frag", Reference{Fragment: "frag", External: true}},
		{"//example.invalid/x", Reference{External: true}},
		{"data:image/png;base64,AAAA", Reference{External: true}},
		{"file:///etc/passwd", Reference{External: true}},
	} {
		got, err := ResolveReference("书/Text/ch.xhtml", Href(tc.href))
		if err != nil || got != tc.want {
			t.Fatalf("%q: %+v, %v; want %+v", tc.href, got, err, tc.want)
		}
	}
	for _, h := range []string{"../../../outside", "/absolute", "%2fabsolute", "%xx", "#%xx", "a%5cb", "a%00b"} {
		if got, err := ResolveReference("书/Text/ch.xhtml", Href(h)); err == nil {
			t.Fatalf("accepted unsafe/invalid %q: %+v", h, got)
		}
	}
}

func TestCollisionAndExactNames(t *testing.T) {
	for _, pair := range [][2]string{{"Text/CH.xhtml", "text/ch.xhtml"}, {"é.xhtml", "e\u0301.xhtml"}, {"Straße/a", "STRASSE/a"}} {
		if CollisionKey(BookPath(pair[0])) != CollisionKey(BookPath(pair[1])) {
			t.Fatal(pair)
		}
	}
	if p, e := Parse("e\u0301 -章.xhtml"); e != nil || string(p) != "e\u0301 -章.xhtml" {
		t.Fatalf("rewrote input %q %v", p, e)
	}
	for _, s := range []string{"../x", "/x", "C:/x", "a\\x", "a//x", "a/./x", "a/../x", "a\x00"} {
		if _, e := Parse(s); e == nil {
			t.Fatal(s)
		}
	}
	for _, s := range []string{strings.Repeat("x", 4097), strings.Repeat("d/", 128) + "file"} {
		if _, e := Parse(s); e == nil {
			t.Fatal("path limit ignored")
		}
	}
}
func FuzzResolve(f *testing.F) {
	for _, s := range []string{"../Text/中文%20章.xhtml", "../../../x", "%2froot", "%252e%252e"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		p, e := Resolve("OEBPS/deep/pkg.opf", Href(s))
		if e == nil {
			if strings.HasPrefix(string(p), "/") || strings.ContainsAny(string(p), "\\:\x00") {
				t.Fatalf("unsafe result %q", p)
			}
			for _, part := range strings.Split(string(p), "/") {
				if part == ".." || part == "." || part == "" {
					t.Fatalf("unsafe component %q", p)
				}
			}
		}
	})
}
