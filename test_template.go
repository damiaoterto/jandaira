package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

func main() {
	tmplStr := `{"content": {{.result | json}}}`
	data := map[string]interface{}{
		"result": "Hello \"world\" \n this is a test.",
		"goal": "Test goal",
	}

	tmpl, err := template.New("body").Funcs(template.FuncMap{
		"json": func(v interface{}) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}).Parse(tmplStr)
	if err != nil {
		fmt.Println("Error parsing:", err)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Println("Error executing:", err)
		return
	}
	fmt.Println("Rendered Output:")
	fmt.Println(buf.String())
}
