package vault

import (
	"errors"
	"strings"
	"testing"
)

const importSecret = "DISTINCTIVE-SECRET-VALUE"

func TestParseImport(t *testing.T) {
	body := []byte(`{
		"version": 1,
		"projects": {
			"empty": {"services": {}},
			"filiz": {
				"services": {
					"vercel": {
						"fields": {
							"email": {"value": "a@b.c", "secret": false},
							"blank": {"value": "", "secret": true}
						}
					}
				}
			}
		}
	}`)
	got, err := ParseImport(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.FieldCount() != 2 {
		t.Fatalf("fields = %d", got.FieldCount())
	}
	if _, ok := got.Projects["empty"]; ok {
		t.Fatal("empty project was created")
	}
	email, err := got.GetField("filiz", "vercel", "email")
	if err != nil {
		t.Fatal(err)
	}
	if email != (Field{Value: "a@b.c", Secret: false}) {
		t.Fatalf("email = %+v", email)
	}
	blank, err := got.GetField("filiz", "vercel", "blank")
	if err != nil {
		t.Fatal(err)
	}
	if blank != (Field{Value: "", Secret: true}) {
		t.Fatalf("blank = %+v", blank)
	}
}

func TestParseImportErrors(t *testing.T) {
	const marker = importSecret
	tests := []struct {
		name    string
		body    string
		want    error
		notWant error
		secret  *MissingSecretError
		caseErr string
		nameErr string
	}{
		{
			name:    "broken",
			body:    `{"version":1,"projects":`,
			want:    ErrInvalidImport,
			notWant: ErrUnsupportedVersion,
		},
		{
			name:    "version with unknown field",
			body:    `{"version":2,"extra":"DISTINCTIVE-SECRET-VALUE","projects":{}}`,
			want:    ErrUnsupportedVersion,
			notWant: ErrInvalidImport,
		},
		{
			name:    "missing version",
			body:    `{"projects":{},"note":"DISTINCTIVE-SECRET-VALUE"}`,
			want:    ErrUnsupportedVersion,
			notWant: ErrInvalidImport,
		},
		{
			name:    "unknown field",
			body:    `{"version":1,"projects":{},"extra":"DISTINCTIVE-SECRET-VALUE"}`,
			want:    ErrInvalidImport,
			notWant: ErrUnsupportedVersion,
		},
		{
			name: "nested unknown field",
			body: `{"version":1,"projects":{"filiz":{"services":{"vercel":{"fields":{"email":{"value":"a@b.c","secret":false,"extra":"DISTINCTIVE-SECRET-VALUE"}}}}}}}`,
			want: ErrInvalidImport,
		},
		{
			name: "trailing value",
			body: `{"version":1,"projects":{}}{"version":1}`,
			want: ErrInvalidImport,
		},
		{
			name:   "missing secret",
			body:   `{"version":1,"projects":{"my-project":{"services":{"vercel":{"fields":{"password":{"value":"DISTINCTIVE-SECRET-VALUE"}}}}}}}`,
			secret: &MissingSecretError{Project: "my-project", Service: "vercel", Field: "password"},
		},
		{
			name:   "null secret",
			body:   `{"version":1,"projects":{"my-project":{"services":{"vercel":{"fields":{"password":{"value":"DISTINCTIVE-SECRET-VALUE","secret":null}}}}}}}`,
			secret: &MissingSecretError{Project: "my-project", Service: "vercel", Field: "password"},
		},
		{
			name:    "case conflict",
			body:    `{"version":1,"projects":{"filiz":{"services":{"vercel":{"fields":{"Email":{"value":"first-secret-value","secret":true},"email":{"value":"DISTINCTIVE-SECRET-VALUE","secret":false}}}}}}}`,
			caseErr: "field",
		},
		{
			name:    "invalid name",
			body:    `{"version":1,"projects":{"a/b":{"services":{"vercel":{"fields":{"email":{"value":"DISTINCTIVE-SECRET-VALUE","secret":true}}}}}}}`,
			nameErr: "contains a path separator",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseImport([]byte(tt.body))
			if tt.secret != nil {
				var got *MissingSecretError
				if !errors.As(err, &got) || *got != *tt.secret {
					t.Fatalf("got %v", err)
				}
			} else if tt.caseErr != "" {
				var got *CaseConflictError
				if !errors.As(err, &got) || got.Kind != tt.caseErr {
					t.Fatalf("got %v", err)
				}
			} else if tt.nameErr != "" {
				var got *NameError
				if !errors.As(err, &got) || got.Reason != tt.nameErr {
					t.Fatalf("got %v", err)
				}
			} else if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if tt.notWant != nil && errors.Is(err, tt.notWant) {
				t.Fatalf("also matched %v", tt.notWant)
			}
			assertNoSecret(t, err, marker, "first-secret-value")
		})
	}
}

func TestParseImportEmpty(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"projects":{}}`,
		`{"version":1,"projects":null}`,
		`{"version":1}`,
		`  {"version":1,"projects":{"p":{"services":{"s":{"fields":{}}}}}}`,
	} {
		got, err := ParseImport([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got.FieldCount() != 0 || len(got.Projects) != 0 {
			t.Fatalf("%s created %+v", body, got.Projects)
		}
	}
}

func TestMergeAddsAndSkips(t *testing.T) {
	dst := New()
	original := Field{Value: "old-secret", Secret: true}
	if err := dst.Add("filiz", "vercel", "password", original); err != nil {
		t.Fatal(err)
	}
	if err := dst.Add("filiz", "vercel", "email", Field{Value: "keep@x", Secret: false}); err != nil {
		t.Fatal(err)
	}
	src := New()
	if err := src.Add("filiz", "vercel", "password", Field{Value: "new-secret", Secret: false}); err != nil {
		t.Fatal(err)
	}
	if err := src.Add("filiz", "vercel", "Email", Field{Value: "case-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := src.Add("filiz", "vercel", "token", Field{Value: "token-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := src.Add("filiz", "digital-ocean", "email", Field{Value: "do@x", Secret: false}); err != nil {
		t.Fatal(err)
	}
	if err := src.Add("other", "github", "pat", Field{Value: "pat-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	beforeSrc := snapshot(t, src)

	got, err := dst.Merge(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Added != 3 {
		t.Fatalf("added = %d", got.Added)
	}
	wantSkipped := []string{
		"filiz/vercel/Email",
		"filiz/vercel/password",
	}
	if strings.Join(got.Skipped, ",") != strings.Join(wantSkipped, ",") {
		t.Fatalf("skipped = %#v", got.Skipped)
	}
	if snapshot(t, src) != beforeSrc {
		t.Fatal("merge changed the source")
	}
	stored, err := dst.GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if stored != original {
		t.Fatalf("password = %+v", stored)
	}
	if _, ok := dst.Projects["filiz"].Services["vercel"].Fields["Email"]; ok {
		t.Fatal("case variant was created")
	}
	token, err := dst.GetField("filiz", "vercel", "token")
	if err != nil {
		t.Fatal(err)
	}
	if token != (Field{Value: "token-secret", Secret: true}) {
		t.Fatalf("token = %+v", token)
	}
	if _, err := dst.GetField("filiz", "digital-ocean", "email"); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.GetField("other", "github", "pat"); err != nil {
		t.Fatal(err)
	}
}

func TestMergeSkipsCaseConflicts(t *testing.T) {
	t.Run("project", func(t *testing.T) {
		dst := New()
		if err := dst.Add("filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}); err != nil {
			t.Fatal(err)
		}
		src := New()
		if err := src.Add("Filiz", "vercel", "token", Field{Value: "project-secret", Secret: true}); err != nil {
			t.Fatal(err)
		}
		if err := src.Add("other", "git", "token", Field{Value: "other-secret", Secret: true}); err != nil {
			t.Fatal(err)
		}
		got, err := dst.Merge(src)
		if err != nil {
			t.Fatal(err)
		}
		if got.Added != 1 || strings.Join(got.Skipped, ",") != "Filiz/vercel/token" {
			t.Fatalf("result = %+v", got)
		}
		if _, ok := dst.Projects["Filiz"]; ok {
			t.Fatal("project case variant was created")
		}
		if _, err := dst.GetField("other", "git", "token"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("service", func(t *testing.T) {
		dst := New()
		if err := dst.Add("filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}); err != nil {
			t.Fatal(err)
		}
		src := New()
		if err := src.Add("filiz", "Vercel", "token", Field{Value: "service-secret", Secret: true}); err != nil {
			t.Fatal(err)
		}
		if err := src.Add("filiz", "digital-ocean", "email", Field{Value: "do@x", Secret: false}); err != nil {
			t.Fatal(err)
		}
		got, err := dst.Merge(src)
		if err != nil {
			t.Fatal(err)
		}
		if got.Added != 1 || strings.Join(got.Skipped, ",") != "filiz/Vercel/token" {
			t.Fatalf("result = %+v", got)
		}
		if _, ok := dst.Projects["filiz"].Services["Vercel"]; ok {
			t.Fatal("service case variant was created")
		}
		if _, err := dst.GetField("filiz", "vercel", "email"); err != nil {
			t.Fatal(err)
		}
		if _, err := dst.GetField("filiz", "digital-ocean", "email"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestMergeKeepsPrefixDistinct(t *testing.T) {
	dst := New()
	if err := dst.Add("filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}); err != nil {
		t.Fatal(err)
	}
	src := New()
	if err := src.Add("fil", "vercel", "email", Field{Value: "other", Secret: true}); err != nil {
		t.Fatal(err)
	}
	got, err := dst.Merge(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Added != 1 || len(got.Skipped) != 0 {
		t.Fatalf("result = %+v", got)
	}
	if _, ok := dst.Projects["fil"]; !ok {
		t.Fatal("prefix was folded into the existing project")
	}
	if _, ok := dst.Projects["filiz"]; !ok {
		t.Fatal("existing project missing")
	}
}

func TestMergeFailureLeavesDestination(t *testing.T) {
	dst := sampleVault(t)
	before := snapshot(t, dst)
	src := New()
	src.Projects = map[string]Project{
		"p": {
			Services: map[string]Service{
				"s": {
					Fields: map[string]Field{
						"Email": {Value: "visible-one", Secret: false},
						"email": {Value: importSecret, Secret: true},
					},
				},
			},
		},
	}
	_, err := dst.Merge(src)
	var conflict *CaseConflictError
	if !errors.As(err, &conflict) || conflict.Kind != "field" {
		t.Fatalf("got %v", err)
	}
	if snapshot(t, dst) != before {
		t.Fatal("failed merge changed the vault")
	}
	assertNoSecret(t, err, importSecret, "visible-one")

	src = New()
	src.Projects = map[string]Project{
		" bad": {
			Services: map[string]Service{
				"vercel": {
					Fields: map[string]Field{
						"email": {Value: importSecret, Secret: true},
					},
				},
			},
		},
	}
	_, err = dst.Merge(src)
	var nameErr *NameError
	if !errors.As(err, &nameErr) {
		t.Fatalf("got %v", err)
	}
	if snapshot(t, dst) != before {
		t.Fatal("invalid merge changed the vault")
	}
	assertNoSecret(t, err, importSecret)
}

func TestMergeEmptyAndNil(t *testing.T) {
	dst := sampleVault(t)
	before := snapshot(t, dst)
	got, err := dst.Merge(New())
	if err != nil {
		t.Fatal(err)
	}
	if got.Added != 0 || len(got.Skipped) != 0 {
		t.Fatalf("result = %+v", got)
	}
	if snapshot(t, dst) != before {
		t.Fatal("empty merge changed the vault")
	}

	_, err = dst.Merge(nil)
	if !errors.Is(err, errNilVault) {
		t.Fatalf("nil source: %v", err)
	}
	if snapshot(t, dst) != before {
		t.Fatal("nil source changed the vault")
	}
	var empty *Vault
	if _, err := empty.Merge(New()); !errors.Is(err, errNilVault) {
		t.Fatalf("nil destination: %v", err)
	}
}
