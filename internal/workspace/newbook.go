package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/validation"
	"golang.org/x/text/language"
)

const definitionLimit = 1 << 20

type NewBook struct {
	SchemaVersion int          `json:"schemaVersion"`
	Title         string       `json:"title"`
	Creator       string       `json:"creator"`
	Language      string       `json:"language"`
	Identifier    string       `json:"identifier"`
	Modified      string       `json:"modified"`
	Chapters      []NewChapter `json:"chapters"`
}

type NewChapter struct {
	Title      string   `json:"title"`
	Paragraphs []string `json:"paragraphs"`
}

type GeneratedSource struct {
	Kind             string            `json:"kind"`
	TemplateVersion  int               `json:"templateVersion"`
	Definition       NewBook           `json:"definition"`
	DefinitionSHA256 string            `json:"definitionSha256"`
	Validation       validation.Report `json:"validation"`
}

// ReadNewBook uses the same nonblocking, no-link file boundary as edit requests.
func ReadNewBook(file string) (NewBook, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return NewBook{}, err
	}
	r, err := openDir(filepath.Dir(abs))
	if err != nil {
		return NewBook{}, err
	}
	defer r.Close()
	var b json.RawMessage
	if err := readEditJSONLimit(r, filepath.Base(abs), &b, definitionLimit); err != nil {
		var f *fault.Error
		if errors.As(err, &f) && f.Code == "WORKSPACE_JSON_LIMIT" {
			return NewBook{}, fault.New(2, "INVALID_ARGUMENT", "definition exceeds %d bytes", definitionLimit)
		}
		return NewBook{}, err
	}
	if err := validDefinitionUnicode(b); err != nil {
		return NewBook{}, err
	}
	var definition NewBook
	if err := decodeStrict(b, &definition); err != nil {
		return definition, err
	}
	return definition, definition.validate()
}

// encoding/json replaces malformed UTF-8 and unpaired UTF-16 escapes. Native
// author input must not silently acquire replacement characters before binding.
func validDefinitionUnicode(b []byte) error {
	if !utf8.Valid(b) {
		return fmt.Errorf("definition must be UTF-8")
	}
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) || b[i] != 'u' {
			continue
		}
		if i+4 >= len(b) {
			return fmt.Errorf("invalid Unicode escape")
		}
		n, err := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		if err != nil {
			return err
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return fmt.Errorf("unpaired Unicode surrogate")
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(b) || string(b[i+1:i+3]) != `\u` {
				return fmt.Errorf("unpaired Unicode surrogate")
			}
			low, err := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return fmt.Errorf("unpaired Unicode surrogate")
			}
			i += 6
		}
	}
	return nil
}

func (b NewBook) validate() error {
	raw, err := json.Marshal(b)
	if err != nil || len(raw) > definitionLimit || b.SchemaVersion != 1 || len(b.Chapters) < 1 || len(b.Chapters) > 64 {
		return fmt.Errorf("definition v1 requires 1–64 chapters within 1 MiB")
	}
	if _, err := language.Parse(b.Language); err != nil || strings.TrimSpace(b.Language) == "" || strings.ContainsAny(b.Language, "_ ") {
		return fmt.Errorf("language must be a BCP47 tag")
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", b.Modified)
	if err != nil || t.Format("2006-01-02T15:04:05Z") != b.Modified {
		return fmt.Errorf("modified must be an exact UTC timestamp")
	}
	texts := []string{b.Title, b.Creator, b.Language, b.Identifier}
	for _, c := range b.Chapters {
		if len(c.Paragraphs) < 1 || len(c.Paragraphs) > 1024 {
			return fmt.Errorf("chapter requires 1–1024 paragraphs")
		}
		texts = append(texts, c.Title)
		texts = append(texts, c.Paragraphs...)
	}
	for _, s := range texts {
		if strings.TrimSpace(s) == "" || !utf8.ValidString(s) {
			return fmt.Errorf("required text must be nonempty valid Unicode")
		}
		for _, r := range s {
			if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff) {
				return fmt.Errorf("text contains an illegal XML character")
			}
		}
	}
	return nil
}

// Template 1 is a persisted provenance algorithm. Changes require a new version.
func (b NewBook) files() map[string]string {
	esc := func(s string) string {
		var out bytes.Buffer
		xml.EscapeText(&out, []byte(s))
		return out.String()
	}
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0" encoding="UTF-8"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>` + "\n",
		"EPUB/style.css":         "body { font-family: serif; writing-mode: horizontal-tb; }\nh1 { text-align: start; }\np { text-indent: 2em; }\n",
	}
	var manifest, spine, links strings.Builder
	for i, c := range b.Chapters {
		id := fmt.Sprintf("chapter-%03d", i+1)
		fmt.Fprintf(&manifest, `<item id="%s" href="%s.xhtml" media-type="application/xhtml+xml"/>`, id, id)
		fmt.Fprintf(&spine, `<itemref idref="%s"/>`, id)
		fmt.Fprintf(&links, `<li><a href="%s.xhtml#start">%s</a></li>`, id, esc(c.Title))
		var paragraphs strings.Builder
		for _, p := range c.Paragraphs {
			fmt.Fprintf(&paragraphs, "<p>%s</p>\n", esc(p))
		}
		files["EPUB/"+id+".xhtml"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" lang="%s" xml:lang="%s"><head><title>%s</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">%s</h1>%s</body></html>`+"\n", esc(b.Language), esc(b.Language), esc(c.Title), esc(c.Title), paragraphs.String())
	}
	files["EPUB/package.opf"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid" xml:lang="%s"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">%s</dc:identifier><dc:title id="title">%s</dc:title><dc:creator id="creator">%s</dc:creator><dc:language>%s</dc:language><meta property="dcterms:modified">%s</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="css" href="style.css" media-type="text/css"/>%s</manifest><spine>%s</spine></package>`+"\n", esc(b.Language), esc(b.Identifier), esc(b.Title), esc(b.Creator), esc(b.Language), b.Modified, manifest.String(), spine.String())
	files["EPUB/nav.xhtml"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="%s" xml:lang="%s"><head><title>Contents</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><nav epub:type="toc" id="toc"><h1>Contents</h1><ol>%s</ol></nav></body></html>`+"\n", esc(b.Language), esc(b.Language), links.String())
	return files
}

func (b NewBook) tree() Tree {
	entries := []Entry{{Path: "EPUB", Type: "directory"}, {Path: "META-INF", Type: "directory"}}
	for name, body := range b.files() {
		h := sha256.Sum256([]byte(body))
		entries = append(entries, Entry{Path: name, Type: "file", Size: int64(len(body)), SHA256: hex.EncodeToString(h[:])})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return Tree{SHA256: hashEntries(entries), Entries: entries}
}

func CreateNew(ctx context.Context, dir string, b NewBook, o validation.Options) (*Workspace, error) {
	if err := b.validate(); err != nil {
		return nil, fault.New(2, "INVALID_ARGUMENT", "%v", err)
	}
	if o.Draft || o.Rootfile != "" {
		return nil, fault.New(2, "INVALID_ARGUMENT", "native creation cannot use draft or rootfile")
	}
	return createInput(dir, "", Options{Context: ctx}, maxJSONBytes, defaultResourceIO(ctx), &b, o)
}

func (w *Workspace) generate(b NewBook, stage string, o validation.Options) (*GeneratedSource, error) {
	for _, dir := range []string{revision, revision + "/META-INF", revision + "/EPUB"} {
		if err := w.root.Mkdir(dir, 0700); err != nil {
			return nil, err
		}
	}
	for name, body := range b.files() {
		if err := w.resources.ctx.Err(); err != nil {
			return nil, err
		}
		if err := w.root.WriteFile(revision+"/"+name, []byte(body), 0600); err != nil {
			return nil, err
		}
	}
	tree, err := w.hashAt(revision)
	if err != nil {
		return nil, err
	}
	report, err := validation.Validate(w.resources.ctx, filepath.Join(stage, filepath.FromSlash(revision)), o)
	if err != nil {
		var f *fault.Error
		if errors.As(err, &f) && f.Code == "CHECKER_TIMEOUT" && errors.Is(w.resources.ctx.Err(), context.Canceled) {
			return nil, errors.Join(context.Canceled, err)
		}
		return nil, err
	}
	if err := approval(report, tree.SHA256); err != nil {
		return nil, err
	}
	return &GeneratedSource{"native-epub3", 1, b, digest(b), report}, nil
}

func (w *Workspace) verifyGenerated() error {
	s := w.state.Source
	if s == nil || s.Kind != "native-epub3" || s.TemplateVersion != 1 || w.state.OriginalSHA256 != "" || w.state.Rootfile != "EPUB/package.opf" || len(w.state.ReadOnlyReasons) != 0 {
		return fmt.Errorf("invalid generated provenance")
	}
	if err := s.Definition.validate(); err != nil {
		return err
	}
	if s.DefinitionSHA256 != digest(s.Definition) || digest(s.Definition.tree()) != digest(w.state.Tree) {
		return fmt.Errorf("generated definition/tree mismatch")
	}
	return approval(s.Validation, w.state.Tree.SHA256)
}
