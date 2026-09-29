package locale

import (
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func TestPreferences(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"nothing", nil, []string{}},
		{"LANG", map[string]string{"LANG": "fr_FR.UTF-8"}, []string{"fr-FR"}},
		{"modifier", map[string]string{"LANG": "fr_FR.UTF-8@euro"}, []string{"fr-FR"}},
		{
			"LC_ALL wins",
			map[string]string{"LC_ALL": "de_DE.UTF-8", "LANG": "fr_FR.UTF-8"},
			[]string{"de-DE"},
		},
		{
			"LC_MESSAGES before LANG",
			map[string]string{"LC_MESSAGES": "es_ES", "LANG": "fr_FR"},
			[]string{"es-ES"},
		},
		{
			"LANGUAGE list first",
			map[string]string{"LANGUAGE": "de:fr", "LANG": "en_US.UTF-8"},
			[]string{"de", "fr", "en-US"},
		},
		{"C locale", map[string]string{"LANG": "C.UTF-8"}, []string{}},
		{"POSIX locale", map[string]string{"LC_ALL": "POSIX"}, []string{}},
		{"empty LANGUAGE entries", map[string]string{"LANGUAGE": "fr::en"}, []string{"fr", "en"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A fake environment: the real one of the process is never touched
			got := Preferences(func(name string) string { return tt.env[name] })
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

var msgHello = &i18n.Message{ID: "TestHello", Other: "Hello {{.Name}}"}

func TestTranslatorFallsBackToEnglish(t *testing.T) {
	tests := []struct {
		name  string
		langs []string
	}{
		{"no preference", nil},
		{"unsupported language", []string{"de-DE"}},
		// A message without French translation, such as this test one
		{"missing translation", []string{"fr"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.langs...).T(msgHello, map[string]any{"Name": "Ada"})
			if got != "Hello Ada" {
				t.Errorf("got %q, want the English text", got)
			}
		})
	}
}

func TestTranslatorPrefersFirstSupported(t *testing.T) {
	msg := &i18n.Message{ID: "TrayQuit", Other: "Quit"}
	if got := New("de", "fr-CA", "en").T(msg, nil); got != "Quitter" {
		t.Errorf("got %q, want the French text: German is not supported", got)
	}
}

func TestPlural(t *testing.T) {
	msg := &i18n.Message{
		ID:    "TrayActiveCount",
		One:   "{{.Count}} active forward",
		Other: "{{.Count}} active forwards",
	}
	fr, en := New("fr"), New()
	tests := []struct {
		count      int
		en, french string
	}{
		{0, "0 active forwards", "0 forward actif"},
		{1, "1 active forward", "1 forward actif"},
		{2, "2 active forwards", "2 forwards actifs"},
		{1_000_000, "1000000 active forwards", "1000000 forwards actifs"},
	}
	for _, tt := range tests {
		if got := en.N(msg, tt.count, nil); got != tt.en {
			t.Errorf("en %d: got %q, want %q", tt.count, got, tt.en)
		}
		if got := fr.N(msg, tt.count, nil); got != tt.french {
			t.Errorf("fr %d: got %q, want %q", tt.count, got, tt.french)
		}
	}
}

// Every English text must be translated: run make i18n-extract, translate
// the translate.*.toml files, then make i18n-merge.
func TestTranslationsComplete(t *testing.T) {
	english := messageIDs(t, "translations/active.en.toml")
	files, err := fs.Glob(translations, "translations/active.*.toml")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	for _, path := range files {
		if path == "translations/active.en.toml" {
			continue
		}
		t.Run(path, func(t *testing.T) {
			translated := messageIDs(t, path)
			for id := range english {
				if !translated[id] {
					t.Errorf("%s is not translated", id)
				}
			}
			for id := range translated {
				if !english[id] {
					t.Errorf("%s is no longer used in the code", id)
				}
			}
		})
	}
}

func messageIDs(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := translations.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var messages map[string]any
	if err := toml.Unmarshal(data, &messages); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ids := make(map[string]bool, len(messages))
	for id := range maps.Keys(messages) {
		ids[id] = true
	}
	if len(ids) == 0 {
		t.Fatalf("%s has no message", strings.TrimPrefix(path, "translations/"))
	}
	return ids
}

// Loading the embedded files must never panic: they are checked here, before
// any release.
func TestEmbeddedFilesLoad(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New panicked: %v", r)
		}
	}()
	New("fr")
}
