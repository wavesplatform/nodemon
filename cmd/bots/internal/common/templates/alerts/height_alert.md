```yaml
Alert type: Height ❌

Level: Error ❌

Details: Some node(s) are {{ .HeightDifference}} blocks behind

{{ with .FirstGroup }}
First group with height {{ .Height}}:

{{range .Nodes}}
{{.}}
{{end}}
{{end}}

{{ with .SecondGroup }}
Second group with height {{ .Height}}:
{{range .Nodes}}
{{.}}
{{end}}
{{end}}
```