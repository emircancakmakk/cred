package vault

import (
	"reflect"
	"testing"
)

func TestCreateEmptyAndRenameHierarchy(t *testing.T) {
	v := New()
	if err := v.CreateProject("p"); err != nil {
		t.Fatal(err)
	}
	if len(v.Projects) != 1 || len(v.Projects["p"].Services) != 0 {
		t.Fatal("empty project missing")
	}
	if err := v.CreateService("p", "s"); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Projects["p"].Services["s"]; !ok {
		t.Fatal("empty service missing")
	}
	field := Field{Value: "secret", Secret: true}
	if err := v.Add("p", "s", "token", field); err != nil {
		t.Fatal(err)
	}
	for _, rename := range []func() error{
		func() error { return v.RenameProject("p", "P") },
		func() error { return v.RenameService("P", "s", "renamed") },
		func() error { return v.RenameField("P", "renamed", "token", "key") },
		func() error { return v.RenameField("P", "renamed", "key", "key") },
	} {
		if err := rename(); err != nil {
			t.Fatal(err)
		}
	}
	got, err := v.GetField("P", "renamed", "key")
	if err != nil || got != field {
		t.Fatalf("descendant lost: %#v %v", got, err)
	}
	if _, ok := v.Projects["p"]; ok {
		t.Fatal("old project still exists")
	}
}

func TestInvalidEditsPreserveVault(t *testing.T) {
	for _, name := range []string{"", " spaced", "a/b", "a\\b", "control\n", "other", "OTHER"} {
		t.Run(name, func(t *testing.T) {
			for _, kind := range []string{"project", "service", "field"} {
				v := New()
				_ = v.Add("p", "s", "token", Field{Value: "keep", Secret: true})
				_ = v.Add("other", "s", "token", Field{})
				_ = v.Add("p", "other", "token", Field{})
				_ = v.Add("p", "s", "other", Field{})
				before := Clone(v)
				var err error
				switch kind {
				case "project":
					err = v.RenameProject("p", name)
				case "service":
					err = v.RenameService("p", "s", name)
				case "field":
					err = v.RenameField("p", "s", "token", name)
				}
				if err == nil || !reflect.DeepEqual(v, before) {
					t.Fatalf("%s %q: err=%v, mutated=%v", kind, name, err, !reflect.DeepEqual(v, before))
				}
			}
		})
	}
}

func TestCreateRejectsDuplicatesAndMissingParent(t *testing.T) {
	v := New()
	_ = v.CreateProject("p")
	_ = v.CreateService("p", "s")
	before := Clone(v)
	for _, create := range []func() error{
		func() error { return v.CreateProject("p") },
		func() error { return v.CreateProject("P") },
		func() error { return v.CreateProject("bad/name") },
		func() error { return v.CreateService("p", "s") },
		func() error { return v.CreateService("p", "S") },
		func() error { return v.CreateService("missing", "new") },
	} {
		if err := create(); err == nil {
			t.Fatal("invalid create succeeded")
		}
		if !reflect.DeepEqual(v, before) {
			t.Fatal("invalid create mutated vault")
		}
	}
}
