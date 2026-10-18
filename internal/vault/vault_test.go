package vault

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateMasterPassword(t *testing.T) {
	twelveWide := strings.Repeat("ş", 12)
	elevenWide := strings.Repeat("ş", 11)
	twelveBytes := strings.Repeat("ş", 6)
	if utf8.RuneCountInString(twelveWide) != 12 || len(twelveWide) <= 12 {
		t.Fatal("fixture must be 12 runes and more than 12 bytes")
	}
	if utf8.RuneCountInString(elevenWide) != 11 || len(elevenWide) <= 12 {
		t.Fatal("fixture must be 11 runes and more than 12 bytes")
	}
	if utf8.RuneCountInString(twelveBytes) >= 12 || len(twelveBytes) != 12 {
		t.Fatal("fixture must be 12 bytes and fewer than 12 runes")
	}

	tests := []struct {
		name     string
		password string
		want     error
	}{
		{name: "12 ascii", password: "123456789012"},
		{name: "11 ascii", password: "12345678901", want: ErrPasswordTooShort},
		{name: "12 multibyte runes", password: twelveWide},
		{name: "11 multibyte runes over 12 bytes", password: elevenWide, want: ErrPasswordTooShort},
		{name: "12 bytes but fewer runes", password: twelveBytes, want: ErrPasswordTooShort},
		{name: "empty", password: "", want: ErrEmptyPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMasterPassword([]byte(tt.password))
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAddCreatesAndMerges(t *testing.T) {
	v := New()
	email := Field{Value: "a@b.c", Secret: false}
	password := Field{Value: "old-secret", Secret: true}
	if err := v.Add("filiz", "vercel", "email", email); err != nil {
		t.Fatal(err)
	}
	if err := v.Add("filiz", "vercel", "password", password); err != nil {
		t.Fatal(err)
	}
	if err := v.Add("filiz", "digital-ocean", "email", email); err != nil {
		t.Fatal(err)
	}

	if got := v.Projects["filiz"].Services["vercel"].Fields["email"]; got != email {
		t.Fatalf("email = %+v", got)
	}
	if got := v.Projects["filiz"].Services["vercel"].Fields["password"]; got != password {
		t.Fatalf("password = %+v", got)
	}
	if _, ok := v.Projects["filiz"].Services["digital-ocean"]; !ok {
		t.Fatal("second service was not created")
	}
}

func TestAddExistingField(t *testing.T) {
	v := New()
	original := Field{Value: "old-secret", Secret: true}
	if err := v.Add("filiz", "vercel", "password", original); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, v)
	err := v.Add("filiz", "vercel", "password", Field{Value: "new-secret", Secret: false})
	if !errors.Is(err, ErrFieldExists) {
		t.Fatalf("got %v", err)
	}
	if snapshot(t, v) != before {
		t.Fatal("vault changed after ErrFieldExists")
	}
	if got := v.Projects["filiz"].Services["vercel"].Fields["password"]; got != original {
		t.Fatalf("field = %+v", got)
	}
}

func TestAddCaseConflict(t *testing.T) {
	v := New()
	if err := v.Add("filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		project string
		service string
		field   string
		kind    string
	}{
		{name: "project", project: "Filiz", service: "vercel", field: "email", kind: "project"},
		{name: "service", project: "filiz", service: "Vercel", field: "token", kind: "service"},
		{name: "field", project: "filiz", service: "vercel", field: "Email", kind: "field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, v)
			err := v.Add(tt.project, tt.service, tt.field, Field{Value: "x", Secret: true})
			var conflict *CaseConflictError
			if !errors.As(err, &conflict) {
				t.Fatalf("got %v", err)
			}
			if conflict.Kind != tt.kind {
				t.Fatalf("kind = %s", conflict.Kind)
			}
			if snapshot(t, v) != before {
				t.Fatal("vault changed after case conflict")
			}
		})
	}
}

func TestAddDoesNotResolvePrefix(t *testing.T) {
	v := New()
	if err := v.Add("filiz", "vercel", "password", Field{Value: "secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := v.Add("fil", "vercel", "password", Field{Value: "other", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Projects["fil"]; !ok {
		t.Fatal("prefix was treated as the existing project")
	}
	if _, ok := v.Projects["filiz"]; !ok {
		t.Fatal("existing project missing")
	}
}

func TestSetReplacesWholeField(t *testing.T) {
	v := New()
	if err := v.Add("filiz", "vercel", "password", Field{Value: "old-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	replacement := Field{Value: "new-secret", Secret: false}
	if err := v.Set("filiz", "vercel", "password", replacement); err != nil {
		t.Fatal(err)
	}
	got, err := v.GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got != replacement {
		t.Fatalf("got %+v, want %+v", got, replacement)
	}
}

func TestSetMissingDoesNotCreate(t *testing.T) {
	v := New()
	err := v.Set("filiz", "vercel", "password", Field{Value: "x", Secret: true})
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing project: %v", err)
	}
	if len(v.Projects) != 0 {
		t.Fatal("Set created a project")
	}

	if err := v.Add("filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, v)
	err = v.Set("filiz", "other", "email", Field{Value: "x", Secret: true})
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("missing service: %v", err)
	}
	err = v.Set("filiz", "vercel", "password", Field{Value: "x", Secret: true})
	if !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("missing field: %v", err)
	}
	if snapshot(t, v) != before {
		t.Fatal("Set created missing records")
	}
}

func TestDeleteField(t *testing.T) {
	v := sampleVault(t)
	if err := v.DeleteField("filiz", "vercel", "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.GetField("filiz", "vercel", "password"); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("password still present: %v", err)
	}
	if _, err := v.GetField("filiz", "vercel", "email"); err != nil {
		t.Fatalf("sibling field removed: %v", err)
	}

	if err := v.DeleteField("filiz", "vercel", "email"); err != nil {
		t.Fatal(err)
	}
	service, ok := v.Projects["filiz"].Services["vercel"]
	if !ok {
		t.Fatal("service removed with its last field")
	}
	if len(service.Fields) != 0 {
		t.Fatalf("fields = %+v", service.Fields)
	}
}

func TestDeleteFieldMissing(t *testing.T) {
	v := sampleVault(t)
	tests := []struct {
		name    string
		project string
		service string
		field   string
		want    error
	}{
		{name: "project", project: "missing", service: "vercel", field: "email", want: ErrProjectNotFound},
		{name: "service", project: "filiz", service: "missing", field: "email", want: ErrServiceNotFound},
		{name: "field", project: "filiz", service: "vercel", field: "missing", want: ErrFieldNotFound},
		{name: "prefix", project: "fil", service: "vercel", field: "email", want: ErrProjectNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, v)
			err := v.DeleteField(tt.project, tt.service, tt.field)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if snapshot(t, v) != before {
				t.Fatal("vault changed")
			}
		})
	}
}

func TestDeleteService(t *testing.T) {
	v := sampleVault(t)
	if err := v.DeleteService("filiz", "vercel"); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Projects["filiz"]; !ok {
		t.Fatal("project removed with its service")
	}
	if _, ok := v.Projects["filiz"].Services["vercel"]; ok {
		t.Fatal("service still present")
	}
	if _, ok := v.Projects["filiz"].Services["digital-ocean"]; !ok {
		t.Fatal("sibling service removed")
	}

	before := snapshot(t, v)
	err := v.DeleteService("filiz", "ver")
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("prefix delete got %v", err)
	}
	if snapshot(t, v) != before {
		t.Fatal("prefix delete changed the vault")
	}
}

func TestDeleteProject(t *testing.T) {
	v := sampleVault(t)
	if err := v.DeleteProject("foo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.Projects["foo"]; ok {
		t.Fatal("project still present")
	}
	if _, ok := v.Projects["filiz"]; !ok {
		t.Fatal("sibling project removed")
	}

	before := snapshot(t, v)
	err := v.DeleteProject("fil")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("prefix delete got %v", err)
	}
	if snapshot(t, v) != before {
		t.Fatal("prefix delete changed the vault")
	}

	err = v.DeleteProject("missing")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("missing delete got %v", err)
	}
}

func TestResolve(t *testing.T) {
	v := sampleVault(t)

	if err := v.Add("fil", "vercel", "password", Field{Value: "x", Secret: true}); err != nil {
		t.Fatal(err)
	}

	foo := v.Projects["foo"]
	foo.Services["GitHub"] = Service{Fields: map[string]Field{
		"token": {Value: "t", Secret: true},
	}}
	v.Projects["foo"] = foo

	vercel := v.Projects["filiz"].Services["vercel"]
	vercel.Fields["Password"] = Field{Value: "case-secret", Secret: true}
	services := v.Projects["filiz"].Services
	services["vercel"] = vercel
	v.Projects["filiz"] = Project{Services: services}

	tests := []struct {
		name    string
		resolve func() (string, error)
		want    string
		wantErr error
		matches []string
	}{
		{
			name:    "exact project wins over prefix",
			resolve: func() (string, error) { return v.ResolveProject("fil") },
			want:    "fil",
		},
		{
			name:    "case insensitive project",
			resolve: func() (string, error) { return v.ResolveProject("FILIZ") },
			want:    "filiz",
		},
		{
			name:    "unique project prefix",
			resolve: func() (string, error) { return v.ResolveProject("fili") },
			want:    "filiz",
		},
		{
			name:    "ambiguous project prefix",
			resolve: func() (string, error) { return v.ResolveProject("f") },
			wantErr: errAmbiguous,
			matches: []string{"fil", "filiz", "foo"},
		},
		{
			name:    "missing project",
			resolve: func() (string, error) { return v.ResolveProject("missing") },
			wantErr: ErrProjectNotFound,
		},
		{
			name:    "case insensitive service",
			resolve: func() (string, error) { return v.ResolveService("filiz", "DIGITAL-OCEAN") },
			want:    "digital-ocean",
		},
		{
			name:    "unique service prefix",
			resolve: func() (string, error) { return v.ResolveService("filiz", "ver") },
			want:    "vercel",
		},
		{
			name:    "missing service",
			resolve: func() (string, error) { return v.ResolveService("filiz", "nope") },
			wantErr: ErrServiceNotFound,
		},
		{
			name:    "service project must be exact",
			resolve: func() (string, error) { return v.ResolveService("fili", "vercel") },
			wantErr: ErrProjectNotFound,
		},
		{
			name:    "case insensitive field",
			resolve: func() (string, error) { return v.ResolveField("filiz", "vercel", "EMAIL") },
			want:    "email",
		},
		{
			name:    "unique field prefix",
			resolve: func() (string, error) { return v.ResolveField("filiz", "vercel", "em") },
			want:    "email",
		},
		{
			name:    "ambiguous field case",
			resolve: func() (string, error) { return v.ResolveField("filiz", "vercel", "PASSWORD") },
			wantErr: errAmbiguous,
			matches: []string{"Password", "password"},
		},
		{
			name:    "exact field wins over other case",
			resolve: func() (string, error) { return v.ResolveField("filiz", "vercel", "password") },
			want:    "password",
		},
		{
			name:    "ambiguous service case",
			resolve: func() (string, error) { return v.ResolveService("foo", "GITHUB") },
			wantErr: errAmbiguous,
			matches: []string{"GitHub", "github"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resolve()
			if tt.wantErr != nil {
				if tt.wantErr == errAmbiguous {
					var ambiguous *AmbiguousError
					if !errors.As(err, &ambiguous) {
						t.Fatalf("got %v", err)
					}
					if strings.Join(ambiguous.Matches, ",") != strings.Join(tt.matches, ",") {
						t.Fatalf("matches = %v, want %v", ambiguous.Matches, tt.matches)
					}
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInvalidNames(t *testing.T) {
	v := New()
	tests := []struct {
		name   string
		value  string
		reason string
	}{
		{name: "empty", value: "", reason: "name is empty"},
		{name: "space", value: " filiz", reason: "leading or trailing space"},
		{name: "slash", value: "a/b", reason: "contains a path separator"},
		{name: "backslash", value: `a\b`, reason: "contains a path separator"},
		{name: "control", value: "a\nb", reason: "contains a control character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Add(tt.value, "vercel", "email", Field{Value: "x", Secret: false})
			var nameErr *NameError
			if !errors.As(err, &nameErr) {
				t.Fatalf("got %v", err)
			}
			if nameErr.Reason != tt.reason {
				t.Fatalf("reason = %q, want %q", nameErr.Reason, tt.reason)
			}
			if len(v.Projects) != 0 {
				t.Fatal("invalid add changed the vault")
			}
		})
	}

	if err := v.Add("digital-ocean", "db_password", "host", Field{Value: "", Secret: false}); err != nil {
		t.Fatalf("valid names and empty value: %v", err)
	}
}

func sampleVault(t *testing.T) *Vault {
	t.Helper()
	v := New()
	entries := []struct {
		project string
		service string
		field   string
		value   Field
	}{
		{"filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}},
		{"filiz", "vercel", "password", Field{Value: "pw", Secret: true}},
		{"filiz", "digital-ocean", "email", Field{Value: "a@b.c", Secret: false}},
		{"foo", "github", "password", Field{Value: "gh", Secret: true}},
	}
	for _, entry := range entries {
		if err := v.Add(entry.project, entry.service, entry.field, entry.value); err != nil {
			t.Fatal(err)
		}
	}
	return v
}

var errAmbiguous = errors.New("ambiguous")
