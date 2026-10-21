package vault

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEmailBatchValidationAndAtomicity(t *testing.T) {
	v := New()
	before := Clone(v)
	batch, err := v.AddSavedEmails("first@EXAMPLE.com, bad address; FIRST@example.com\nsecond@example.com\r\n")
	if err == nil || len(batch.Added) != 2 || len(batch.Skipped) != 1 || len(batch.Invalid) != 1 || !reflect.DeepEqual(v, before) {
		t.Fatalf("invalid batch changed vault: %+v %v", batch, err)
	}
	batch, err = v.AddSavedEmails("first@EXAMPLE.com, FIRST@example.com;second@example.com\r\n ; ,")
	if err != nil || len(batch.Added) != 2 || len(batch.Skipped) != 1 || len(v.SavedEmails) != 2 || v.Version != 2 {
		t.Fatalf("batch: %+v %v", batch, err)
	}
	before = Clone(v)
	batch, err = v.AddSavedEmails("FIRST@example.com; second@example.com")
	if err != nil || len(batch.Added) != 0 || len(batch.Skipped) != 2 || !reflect.DeepEqual(v, before) {
		t.Fatal("duplicates changed vault")
	}
	unicodeBatch := PreviewEmails("Σ@example.com;ς@example.com", nil)
	if len(unicodeBatch.Added) != 1 || len(unicodeBatch.Skipped) != 1 {
		t.Fatal("inconsistent Unicode duplicate handling")
	}
	v = New()
	if _, err = v.AddSavedEmails(" , ;\n\t"); err != nil || v.Version != 1 || len(v.SavedEmails) != 0 {
		t.Fatal("empty batch upgraded vault")
	}
}
func TestNormalizeEmail(t *testing.T) {
	for _, value := range []string{"", "no-at", "a@@example.com", "a b@example.com", "Name <a@example.com>", "a@example.com (comment)", "\"quoted\"@example.com", "a@exa\x1bmple.com", "a@example.com\nother@example.com"} {
		if _, err := NormalizeEmail(value); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("accepted %q: %v", value, err)
		}
	}
	for input, want := range map[string]string{" A+tag@EXAMPLE.COM ": "A+tag@example.com", "person@example.com": "person@example.com"} {
		got, err := NormalizeEmail(input)
		if err != nil || got != want {
			t.Errorf("%q -> %q %v", input, got, err)
		}
	}
}
func TestSavedEmailCRUDCopyAndPersistence(t *testing.T) {
	v := New()
	_ = v.Add("p", "s", "email", Field{Value: "first@example.com"})
	_, err := v.AddSavedEmails("first@example.com;second@example.com")
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for key, address := range v.SavedEmails {
		if address == "first@example.com" {
			id = key
		}
	}
	before := Clone(v)
	for _, input := range []string{"invalid", "SECOND@example.com"} {
		if err = v.EditSavedEmail(id, input); err == nil || !reflect.DeepEqual(v, before) {
			t.Fatal("invalid edit changed vault")
		}
	}
	if err = v.EditSavedEmail(id, "changed@EXAMPLE.COM"); err != nil || v.SavedEmails[id] != "changed@example.com" || before.SavedEmails[id] != "first@example.com" {
		t.Fatal("edit or clone failed")
	}
	path := filepath.Join(t.TempDir(), "vault.age")
	if err = Save(path, []byte("test"), v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, []byte("test"))
	if err != nil || !reflect.DeepEqual(v, got) {
		t.Fatalf("round trip: %v", err)
	}
	src := New()
	_ = src.Add("p", "s", "token", Field{Value: "new"})
	if _, err = got.Merge(src); err != nil || !reflect.DeepEqual(got.SavedEmails, v.SavedEmails) {
		t.Fatal("import lost collection")
	}
	for key := range got.SavedEmails {
		if err = got.DeleteSavedEmail(key); err != nil {
			t.Fatal(err)
		}
	}
	if got.Version != 2 {
		t.Fatal("last deletion downgraded vault")
	}
	f, _ := got.GetField("p", "s", "email")
	if f.Value != "first@example.com" {
		t.Fatal("collection changed copied field")
	}
	if err = got.DeleteSavedEmail(id); !errors.Is(err, ErrEmailNotFound) {
		t.Fatal(err)
	}
	if err = got.EditSavedEmail(id, "a@example.com"); !errors.Is(err, ErrEmailNotFound) {
		t.Fatal(err)
	}
	if err = Save(path, []byte("test"), got, fastOptions()); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path, []byte("test"))
	if err != nil || got.Version != 2 || len(got.SavedEmails) != 0 {
		t.Fatal("empty v2 load failed")
	}
}
func TestEmailStorageRejectsMalformedCollections(t *testing.T) {
	const id = "00000000000000000000000000000001"
	for _, body := range []string{
		`{"version":1,"projects":{},"saved_emails":{"` + id + `":"a@example.com"}}`,
		`{"version":2,"saved_emails":{"bad-id":"a@example.com"}}`,
		`{"version":2,"saved_emails":{"` + id + `":"invalid"}}`,
		`{"version":2,"saved_emails":{"` + id + `":"a@EXAMPLE.COM"}}`,
		`{"version":2,"saved_emails":{"` + id + `":"a@example.com","00000000000000000000000000000002":"A@example.com"}}`,
	} {
		if _, err := parsePlaintext([]byte(body)); !errors.Is(err, ErrCorruptVault) {
			t.Errorf("accepted malformed collection: %v", err)
		}
	}
}
func TestReconcileSavedEmails(t *testing.T) {
	const id = "00000000000000000000000000000001"
	base := New()
	_, _ = base.AddSavedEmails("a@example.com;b@example.com")
	ids := sortedKeys(base.SavedEmails)
	unexpected := func(string) (bool, error) { t.Fatal("unexpected conflict"); return false, nil }
	l, r := Clone(base), Clone(base)
	_ = l.EditSavedEmail(ids[0], "local@example.com")
	_ = r.DeleteSavedEmail(ids[1])
	merged, err := Reconcile(base, l, r, unexpected)
	if err != nil || len(merged.SavedEmails) != 1 || merged.SavedEmails[ids[0]] != "local@example.com" {
		t.Fatalf("independent edits: %+v %v", merged, err)
	}
	for _, deletion := range []bool{false, true} {
		for _, mine := range []bool{false, true} {
			l, r = Clone(base), Clone(base)
			_ = l.EditSavedEmail(ids[0], "local@example.com")
			if deletion {
				_ = r.DeleteSavedEmail(ids[0])
			} else {
				_ = r.EditSavedEmail(ids[0], "remote@example.com")
			}
			calls := 0
			merged, err = Reconcile(base, l, r, func(path string) (bool, error) {
				calls++
				if path != "saved emails/"+ids[0] || strings.Contains(path, "@") {
					t.Fatal("unsafe conflict label")
				}
				return mine, nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("conflict: %v %d", err, calls)
			}
			want := r.SavedEmails[ids[0]]
			if mine {
				want = l.SavedEmails[ids[0]]
			}
			if merged.SavedEmails[ids[0]] != want {
				t.Fatal("wrong conflict resolution")
			}
		}
	}
	cancelled := errors.New("cancel")
	if _, err = Reconcile(base, l, r, func(string) (bool, error) { return false, cancelled }); !errors.Is(err, cancelled) {
		t.Fatal("cancel ignored")
	}
	// A v1 writer's unrelated change must retain a concurrent v2 collection.
	base = New()
	l = Clone(base)
	r = Clone(base)
	_ = l.Add("p", "s", "f", Field{Value: "local"})
	_, _ = r.AddSavedEmails("same@example.com")
	merged, err = Reconcile(base, l, r, unexpected)
	if err != nil || merged.Version != 2 || len(merged.Projects) != 1 || len(merged.SavedEmails) != 1 {
		t.Fatal("mixed versions lost data")
	}
	// Duplicate concurrent additions converge in either merge direction.
	l, r = Clone(base), Clone(base)
	l.Version, r.Version = 2, 2
	l.SavedEmails = map[string]string{id: "same@example.com"}
	r.SavedEmails = map[string]string{"00000000000000000000000000000002": "SAME@example.com"}
	one, err := Reconcile(base, l, r, unexpected)
	if err != nil {
		t.Fatal(err)
	}
	two, err := Reconcile(base, r, l, unexpected)
	if err != nil || !reflect.DeepEqual(one, two) || len(one.SavedEmails) != 1 || one.SavedEmails[id] != "same@example.com" {
		t.Fatal("duplicate merge not deterministic")
	}
}
