package main

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates
var f embed.FS

func getTemplate(name string) (*template.Template, error) {
	text, err := f.ReadFile(fmt.Sprintf("templates/%s.gohtml", name))
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).Parse(string(text))
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}
