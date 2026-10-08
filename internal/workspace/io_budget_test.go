package workspace

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
)

type r3HookContext struct {
	context.Context
	check func()
}

func (c r3HookContext) Err() error { c.check(); return c.Context.Err() }

func r3Limit(t *testing.T, err error, code string) {
	t.Helper()
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestR3TreeBudgets(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "src/a"), []byte("12345"))
	put(t, filepath.Join(root, "src/书/b"), []byte("67"))
	r, err := openDir(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	wire := fmt.Sprintf(`[{"path":"a","type":"file","size":5,"sha256":"%x"},{"path":"书","type":"directory","size":0},{"path":"书/b","type":"file","size":2,"sha256":"%x"}]`, sha256.Sum256([]byte("12345")), sha256.Sum256([]byte("67")))
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("kepub-tree-v1\n"+wire)))
	for _, kind := range []string{"file", "total", "entries", "paths"} {
		for _, delta := range []int64{-1, 0, 1} {
			t.Run(fmt.Sprintf("%s/%d", kind, delta), func(t *testing.T) {
				policy := defaultResourceIO(t.Context())
				policy.limits = archive.Limits{Entries: 3, FileBytes: 5, TotalBytes: 7, PathBytes: 9}
				switch kind {
				case "file":
					policy.limits.FileBytes += delta
				case "total":
					policy.limits.TotalBytes += delta
				case "entries":
					policy.limits.Entries += int(delta)
				case "paths":
					policy.limits.PathBytes += delta
				}
				dest := "copy-" + kind + fmt.Sprint(delta)
				hash, hashErr := hashAtWithIO(r, "src", policy)
				copy, copyErr := copyTreeWithIO(r, "src", dest, policy)
				if delta < 0 {
					r3Limit(t, hashErr, "ARCHIVE_LIMIT")
					r3Limit(t, copyErr, "ARCHIVE_LIMIT")
					if exists(r, dest) {
						t.Fatal("partial tree retained after refusal")
					}
				} else {
					if hashErr != nil || copyErr != nil || hash.SHA256 != want || copy.SHA256 != want || len(copy.Entries) != 3 {
						t.Fatal("exact legal inventory/hash changed", hash, copy, hashErr, copyErr)
					}
					assertBytes(t, filepath.Join(root, dest, "a"), []byte("12345"))
					assertBytes(t, filepath.Join(root, dest, "书/b"), []byte("67"))
				}
				assertBytes(t, filepath.Join(root, "src/a"), []byte("12345"))
				assertBytes(t, filepath.Join(root, "src/书/b"), []byte("67"))
			})
		}
	}
}

func TestR3OriginalBudget(t *testing.T) {
	for _, size := range []int{14, 15, 16} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			data := []byte(strings.Repeat("x", size)) // Not a ZIP; reject before parsing.
			put(t, source, data)
			if err := os.MkdirAll(filepath.Join(root, "dst/original"), 0700); err != nil {
				t.Fatal(err)
			}
			dst, err := openDir(filepath.Join(root, "dst"))
			if err != nil {
				t.Fatal(err)
			}
			defer dst.Close()
			policy := defaultResourceIO(t.Context())
			policy.originalBytes = 15
			hash, err := copyOriginal(dst, source, policy)
			if size > 15 {
				r3Limit(t, err, "ARCHIVE_LIMIT")
				if exists(dst, "original/book.epub") {
					t.Fatal("oversize original copied before refusal")
				}
				ws := filepath.Join(root, "ws")
				if w, err := createWithIO(ws, source, Options{}, maxJSONBytes, policy); w != nil {
					w.Close()
					t.Fatal("oversize non-ZIP published")
				} else {
					r3Limit(t, err, "ARCHIVE_LIMIT")
				}
				if _, err := os.Lstat(ws); !os.IsNotExist(err) {
					t.Fatal("partial workspace published", err)
				}
				stages, _ := filepath.Glob(filepath.Join(root, ".kepub-create-*"))
				if len(stages) != 0 {
					t.Fatal("Create staging leaked", stages)
				}
			} else {
				if err != nil || hash != fmt.Sprintf("%x", sha256.Sum256(data)) {
					t.Fatal(hash, err)
				}
				assertBytes(t, filepath.Join(root, "dst/original/book.epub"), data)
			}
			assertBytes(t, source, data)
		})
	}
	// Expanded bytes/entries/paths come from the independent fixture inventory,
	// not the ZIP length. ZIP headers make this tiny raw stream larger than its
	// complete expanded file inventory; the two budgets must stay separate.
	source, files := fixture(t)
	data := readResource(t, source)
	policy := defaultResourceIO(t.Context())
	policy.originalBytes = int64(len(data))
	policy.limits = archive.Limits{}
	names := map[string]bool{}
	for _, entry := range files {
		name := strings.TrimSuffix(entry.Name, "/")
		policy.limits.TotalBytes += int64(len(entry.Data))
		policy.limits.FileBytes = max(policy.limits.FileBytes, int64(len(entry.Data)))
		for p := name; p != "."; p = path.Dir(p) {
			names[p] = true
		}
	}
	policy.limits.Entries = len(names)
	for name := range names {
		policy.limits.PathBytes += int64(len(name))
	}
	if policy.originalBytes == policy.limits.TotalBytes {
		t.Fatal("fixture does not distinguish budgets")
	}
	dir := filepath.Join(t.TempDir(), "ws")
	w, err := createWithIO(dir, source, Options{}, maxJSONBytes, policy)
	if err != nil {
		t.Fatal("valid ZIP rejected by mixed raw/expanded budgets", err)
	}
	w.Close()
	w, err = openWithIO(dir, policy)
	if err != nil {
		t.Fatal("bounded original verification failed", err)
	}
	w.Close()
	for _, entry := range files {
		if entry.Mode.IsDir() {
			info, err := os.Stat(filepath.Join(dir, revision, entry.Name))
			if err != nil || !info.IsDir() {
				t.Fatal("directory inventory lost", err)
			}
		} else {
			assertBytes(t, filepath.Join(dir, revision, entry.Name), entry.Data)
		}
	}
	assertBytes(t, source, data)
}

func TestR3GrowthAfterStat(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, limit := range []int64{8, 16} {
			t.Run(fmt.Sprintf("raw=%t/limit=%d", raw, limit), func(t *testing.T) {
				root := t.TempDir()
				source := filepath.Join(root, "src/a")
				put(t, source, []byte("abcd"))
				dest := filepath.Join(root, "copy/a")
				if raw {
					if err := os.MkdirAll(filepath.Join(root, "original"), 0700); err != nil {
						t.Fatal(err)
					}
					dest = filepath.Join(root, "original/book.epub")
				}
				r, err := openDir(root)
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				grew := false
				policy := defaultResourceIO(r3HookContext{t.Context(), func() {
					if _, err := os.Lstat(dest); err == nil && !grew {
						grew = true
						f, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
						if err != nil {
							t.Fatal(err)
						}
						_, err = f.Write([]byte("efghijklm"))
						if err = errors.Join(err, f.Close()); err != nil {
							t.Fatal(err)
						}
					}
				}})
				policy.originalBytes = limit
				policy.limits.FileBytes = limit
				if raw {
					_, err = copyOriginal(r, source, policy)
				} else {
					_, err = copyTreeWithIO(r, "src", "copy", policy)
				}
				code := "ARCHIVE_LIMIT"
				if limit == 16 {
					code = "INPUT_DRIFT"
				}
				r3Limit(t, err, code)
				if !grew {
					t.Fatal("growth window not executed")
				}
				if _, err := os.Lstat(dest); !os.IsNotExist(err) {
					t.Fatal("partial grown copy retained", err)
				}
				assertBytes(t, source, []byte("abcdefghijklm")) // Only the test appends.
			})
		}
	}
}

func TestR3PartialTreeFailure(t *testing.T) {
	for _, kind := range []string{"cancel", "second-destination"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			put(t, filepath.Join(root, "src/a"), []byte("first"))
			put(t, filepath.Join(root, "src/b"), []byte("last"))
			r, err := openDir(root)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			triggered := false
			policy := defaultResourceIO(r3HookContext{ctx, func() {
				entries, _ := os.ReadDir(filepath.Join(root, "copy"))
				if triggered || len(entries) != 1 {
					return
				}
				info, err := entries[0].Info()
				if err != nil || info.Size() == 0 {
					return
				}
				triggered = true
				if kind == "cancel" {
					cancel()
					return
				}
				other := "a"
				if entries[0].Name() == "a" {
					other = "b"
				}
				put(t, filepath.Join(root, "copy", other), []byte("injected EEXIST"))
			}})
			_, err = copyTreeWithIO(r, "src", "copy", policy)
			if !triggered || kind == "cancel" && !errors.Is(err, context.Canceled) || kind != "cancel" && !errors.Is(err, os.ErrExist) {
				t.Fatal("wrong failure window", kind, triggered, err)
			}
			if exists(r, "copy") {
				t.Fatal("partial multi-file copy leaked")
			}
			assertBytes(t, filepath.Join(root, "src/a"), []byte("first"))
			assertBytes(t, filepath.Join(root, "src/b"), []byte("last"))
		})
	}
}

func TestR3CancelledRunningRollback(t *testing.T) {
	w, dir, source := multiWorkspace(t)
	defer func() { w.Close() }()
	p := multiWritePlan(t, w)
	e, outputs := prepareExecution(t, w, p)
	before := n1Tree(t, filepath.Join(dir, revision))
	book := readResource(t, source)
	first := filepath.Join(dir, candidate, p.WriteSet[0])
	second := filepath.Join(dir, candidate, p.WriteSet[1])
	oldFirst, oldSecond := readResource(t, first), readResource(t, second)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	triggered := false
	w.resources.ctx = r3HookContext{ctx, func() {
		if !triggered && !reflect.DeepEqual(readResource(t, first), oldFirst) {
			triggered = true
			assertBytes(t, second, oldSecond)
			cancel()
		}
	}}
	failed, err := w.execute(e, outputs, nil)
	if !triggered || !errors.Is(err, context.Canceled) || failed.Status != "failed" || failed.ReviewRequired {
		t.Fatal("registered cancellation lost rollback", triggered, failed, err)
	}
	if after := n1Tree(t, filepath.Join(dir, candidate)); !reflect.DeepEqual(before, after) {
		t.Fatal("rollback lost publication bytes")
	}
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
	assertBytes(t, source, book)
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("lock/recovery not reusable", err)
	}
	defer w.Close()
	if got, err := w.Execution(); err != nil || got.Status != "failed" {
		t.Fatal("failure audit not recoverable", got, err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestR3CommittedRecoveryRetainsBudget(t *testing.T) {
	w, dir := makeWorkspace(t)
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	s, err := w.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	before := n1Tree(t, filepath.Join(dir, revision))
	book := readResource(t, filepath.Join(dir, "original/book.epub"))
	put(t, filepath.Join(pub, "outside.txt"), []byte("uncheckpointed"))
	if _, err := w.copyTree(checkpointDir(s.ID)+"/pub", restoreNew); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.root, restoreJournal, restoreRecord{1, s.ID, s.Tree.SHA256}); err != nil {
		t.Fatal(err)
	}
	journal := readResource(t, filepath.Join(dir, restoreJournal))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	w.resources.ctx = ctx
	// A durable journal ignores late cancellation, but not the same resource
	// limits or real I/O failure. Two interrupted recoveries must retain intent.
	limits := w.resources.limits
	w.resources.limits.Entries = 0
	r3Limit(t, w.recoverRestore(), "ARCHIVE_LIMIT")
	w.resources.limits = limits
	blocked := filepath.Join(dir, "tasks/active/work")
	if err := os.Chmod(blocked, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0700) })
	for i := 0; i < 2; i++ {
		if err := w.recoverRestore(); !errors.Is(err, os.ErrPermission) {
			t.Fatal("expected real recovery I/O failure", err)
		}
		assertBytes(t, filepath.Join(dir, restoreJournal), journal)
		assertBytes(t, filepath.Join(pub, "outside.txt"), []byte("uncheckpointed"))
	}
	if err := os.Chmod(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("durable recovery retry/lock acquisition", err)
	}
	defer w.Close()
	if got := n1Tree(t, pub); !reflect.DeepEqual(got, before) {
		t.Fatal("recovered tree differs from independent frozen bytes")
	}
	assertBytes(t, filepath.Join(dir, "original/book.epub"), book)
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
}

func TestR3DiffSnapshotCancellation(t *testing.T) {
	w, dir, original := legalWorkspace(t, "3.0")
	defer w.Close()
	book := readResource(t, original)
	e := applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "New title"))
	before := n1Tree(t, dir)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	triggered := false
	w.resources.ctx = r3HookContext{ctx, func() {
		stages, err := filepath.Glob(filepath.Join(tmp, "kepub-snapshot-*"))
		if err != nil {
			t.Fatal(err)
		}
		if len(stages) != 0 {
			triggered = true
			cancel()
		}
	}}
	_, err := w.TaskDiff(e.TaskID)
	if !triggered || !errors.Is(err, context.Canceled) {
		t.Fatal("snapshot cancellation was downgraded to successful partial review", triggered, err)
	}
	assertEmpty(t, tmp)
	if !reflect.DeepEqual(before, n1Tree(t, dir)) || w.current != "initial" {
		t.Fatal("cancelled diff changed workspace bytes or accepted")
	}
	assertBytes(t, original, book)
}
