package vault

import (
	"errors"
	"testing"
)

func TestReconcile(t *testing.T) {
	base := New()
	_ = base.Add("p", "s", "a", Field{Value: "old"})
	_ = base.Add("p", "s", "b", Field{Value: "old"})
	tests := []struct {
		name      string
		change    func(*Vault, *Vault)
		conflicts int
		mine      bool
		wantA     string
		deleted   bool
	}{
		{"different fields", func(l, r *Vault) {
			_ = l.Set("p", "s", "a", Field{Value: "local"})
			_ = r.Set("p", "s", "b", Field{Value: "remote"})
		}, 0, false, "local", false},
		{"same field team", func(l, r *Vault) {
			_ = l.Set("p", "s", "a", Field{Value: "local"})
			_ = r.Set("p", "s", "a", Field{Value: "remote"})
		}, 1, false, "remote", false},
		{"same field mine", func(l, r *Vault) {
			_ = l.Set("p", "s", "a", Field{Value: "local"})
			_ = r.Set("p", "s", "a", Field{Value: "remote"})
		}, 1, true, "local", false},
		{"same result", func(l, r *Vault) {
			_ = l.Set("p", "s", "a", Field{Value: "same"})
			_ = r.Set("p", "s", "a", Field{Value: "same"})
		}, 0, false, "same", false},
		{"delete project versus edit", func(l, r *Vault) { _ = l.DeleteProject("p"); _ = r.Set("p", "s", "a", Field{Value: "remote"}) }, 1, true, "", true},
		{"delete service versus edit", func(l, r *Vault) { _ = l.DeleteService("p", "s"); _ = r.Set("p", "s", "a", Field{Value: "remote"}) }, 1, false, "remote", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, r := Clone(base), Clone(base)
			tt.change(l, r)
			n := 0
			got, e := Reconcile(base, l, r, func(path string) (bool, error) { n++; return tt.mine, nil })
			if e != nil {
				t.Fatal(e)
			}
			if n != tt.conflicts {
				t.Fatalf("conflicts=%d", n)
			}
			if tt.deleted {
				if len(got.Projects) != 0 {
					t.Fatal("project retained")
				}
				return
			}
			f, e := got.GetField("p", "s", "a")
			if e != nil || f.Value != tt.wantA {
				t.Fatalf("field=%v err=%v", f, e)
			}
			if tt.name == "different fields" {
				f, _ := got.GetField("p", "s", "b")
				if f.Value != "remote" {
					t.Fatal("remote edit lost")
				}
			}
		})
	}
}
func TestReconcileCancelAndCaseCollision(t *testing.T) {
	b := New()
	l, r := New(), New()
	_ = l.Add("p", "s", "a", Field{Value: "l"})
	_ = r.Add("p", "s", "a", Field{Value: "r"})
	cancelled := errors.New("cancel")
	if _, e := Reconcile(b, l, r, func(string) (bool, error) { return false, cancelled }); !errors.Is(e, cancelled) {
		t.Fatal(e)
	}
	r = New()
	_ = r.Add("P", "s", "b", Field{})
	if _, e := Reconcile(b, l, r, func(string) (bool, error) { t.Fatal("unexpected conflict"); return false, nil }); e == nil {
		t.Fatal("case collision accepted")
	}
}
