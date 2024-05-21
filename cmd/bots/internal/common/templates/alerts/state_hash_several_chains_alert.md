```yaml
🔱 Nodes are on different chains at height {{ .SameHeight}}
{{ with .FirstGroup }}
BlockID (First group): {{ .BlockID}}{{range .Nodes}}
{{.}}{{end}}{{end}}
{{ with .SecondGroup }}
BlockID (Second group): {{ .BlockID}}{{range .Nodes}}
{{.}}{{end}}{{end}}
{{ if .LastCommonStateHashExist }}
Last common Block: {{ .ForkBlockID}} at {{ .ForkHeight}}{{ end }}
```
