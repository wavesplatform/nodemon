```yaml
Alert type: State Hash ❌

Level: Error ❌

Details: Nodes have different state hashes at the same height {{ .SameHeight}}
{{ with .FirstGroup }}
BlockID (First group): {{ .BlockID}}
State Hash (First group): {{ .StateHash}}
{{range .Nodes}}
{{.}}
{{end}}
{{end}}
{{ with .SecondGroup }}
BlockID (Second group): {{ .BlockID}}
State Hash (Second group): {{ .StateHash}}
{{range .Nodes}}
{{.}}
{{end}}
{{end}}
{{ if .LastCommonStateHashExist }}
Fork occurred after block {{ .ForkHeight}}
BlockID: {{ .ForkBlockID}}
State Hash: {{ .ForkStateHash}}
{{ end }}
```
