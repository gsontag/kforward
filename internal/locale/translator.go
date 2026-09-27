// Package locale translates the interface into the language of the user.
// English is the default: its texts live in the code, next to their use;
// other languages come from the translation files embedded in the binary.
package locale

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed translations/active.*.toml
var translations embed.FS

// Translator renders messages in one language. It is safe for concurrent use.
type Translator struct {
	localizer *i18n.Localizer
}

// New returns a translator for the first supported language of langs, in
// order of preference; English when none is supported.
func New(langs ...string) *Translator {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	files, _ := fs.Glob(translations, "translations/active.*.toml")
	for _, path := range files {
		// The English texts are those of the code: a stale file must not win
		if path == "translations/active.en.toml" {
			continue
		}
		if _, err := bundle.LoadMessageFileFS(translations, path); err != nil {
			// Embedded at build time: a broken file is a bug, caught by the tests
			panic(fmt.Sprintf("locale: %v", err))
		}
	}
	return &Translator{localizer: i18n.NewLocalizer(bundle, langs...)}
}

// FromEnvironment returns a translator for the language of the session, as
// set by LANGUAGE, LC_ALL, LC_MESSAGES and LANG.
func FromEnvironment() *Translator {
	return New(Preferences(os.Getenv)...)
}

// Preferences lists the languages of the environment, most preferred first,
// as BCP 47 tags: "fr_FR.UTF-8" becomes "fr-FR".
func Preferences(getenv func(string) string) []string {
	var langs []string
	if list := getenv("LANGUAGE"); list != "" {
		langs = append(langs, strings.Split(list, ":")...)
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		// The first one set wins, as in the C library
		if value := getenv(name); value != "" {
			langs = append(langs, value)
			break
		}
	}

	tags := make([]string, 0, len(langs))
	for _, l := range langs {
		// Drop the encoding and the modifier: fr_FR.UTF-8@euro
		l, _, _ = strings.Cut(l, ".")
		l, _, _ = strings.Cut(l, "@")
		if l == "" || l == "C" || l == "POSIX" {
			continue
		}
		tags = append(tags, strings.ReplaceAll(l, "_", "-"))
	}
	return tags
}

// T renders msg; data fills its {{.Field}} placeholders.
func (t *Translator) T(msg *i18n.Message, data any) string {
	return t.localize(&i18n.LocalizeConfig{DefaultMessage: msg, TemplateData: data})
}

// N renders msg in the plural form matching count, available to the
// template as {{.Count}}, along with the fields of data.
func (t *Translator) N(msg *i18n.Message, count int, data map[string]any) string {
	values := map[string]any{"Count": count}
	for k, v := range data {
		values[k] = v
	}
	return t.localize(
		&i18n.LocalizeConfig{DefaultMessage: msg, PluralCount: count, TemplateData: values},
	)
}

func (t *Translator) localize(config *i18n.LocalizeConfig) string {
	// A missing translation still renders the English text: never fail the UI
	text, _ := t.localizer.Localize(config)
	return text
}
