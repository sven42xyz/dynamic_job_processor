package tmpl

import (
	"bytes"
	"fmt"
	"html/template"

	"djp.chapter42.de/a/internal/data"
)

func PrepareTemplates(cfg *data.WavelyConfig) error {
	current := &cfg.Current // Pointer nötig, um Änderungen zu speichern

	checkTpl, err := template.New("check").Parse(current.Endpoints.Check)
	if err != nil {
		return fmt.Errorf("error in check endpoint template [%s]: %w", current.Name, err)
	}
	writeTpl, err := template.New("write").Parse(current.Endpoints.Write)
	if err != nil {
		return fmt.Errorf("error in write endpoint template [%s]: %w", current.Name, err)
	}

	current.ParsedCheckTpl = checkTpl
	current.ParsedWriteTpl = writeTpl

	return nil
}

func RenderEndpoint(tpl *template.Template, job data.Job) (string, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, job); err != nil {
		return "", err
	}
	return buf.String(), nil
}
