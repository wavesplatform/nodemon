```yaml
Alert type: Several Chains Detected ❌
Details: Nodes have different state hashes at the same height {{ .SameHeight}}
{{ with .FirstGroup }}
BlockID (First group): {{ .BlockID}}
{{range .Nodes}}
{{.}}
{{end}}
{{end}}
{{ with .SecondGroup }}
BlockID (Second group): {{ .BlockID}}
{{range .Nodes}}
{{.}}
{{end}}
{{end}}
{{ if .LastCommonStateHashExist }}
Last common Block: {{ .ForkBlockID}} at {{ .ForkHeight}}
{{ end }}
```
