package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed locales/*
var locales embed.FS

var (
	localizer  *i18n.Localizer
	titleCaser cases.Caser
)

// setupI18n loads the embedded translation bundle and builds a localizer for lang.
func setupI18n(lang string) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, f := range []string{"locales/en.json", "locales/fr.json"} {
		if _, err := bundle.LoadMessageFileFS(locales, f); err != nil {
			panic(fmt.Sprintf("loading %s: %v", f, err))
		}
	}
	localizer = i18n.NewLocalizer(bundle, lang)

	tag, err := language.Parse(lang)
	if err != nil {
		tag = language.French
	}
	titleCaser = cases.Title(tag)
}

// logf prints an already-localized line to the standard logger.
func logf(msg string) { log.Println(msg) }

// tr localizes a message ID, optionally with template data.
func tr(id string, data ...map[string]any) string {
	cfg := &i18n.LocalizeConfig{MessageID: id}
	if len(data) > 0 {
		cfg.TemplateData = data[0]
	}
	return localizer.MustLocalize(cfg)
}

// translateLabel localizes an object label (e.g. "person"), falling back to the
// title-cased raw label when no translation exists.
func translateLabel(label string) string {
	s, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "label_" + label})
	if err != nil {
		return titleCaser.String(label)
	}
	return s
}
