```yaml
❌ State Hash Alert
Nodes on the same chain have diverging state hashes at {{ .SameHeight}}
{{ with .FirstGroup }}
State Hash (First group): {{ .StateHash}}{{range .Nodes}}
{{.}}{{end}}{{end}}
{{ with .SecondGroup }}
State Hash (Second group): {{ .StateHash}}{{range .Nodes}}
{{.}}{{end}}{{end}}
{{ if .LastCommonStateHashExist }}
Fork occurred after block {{ .ForkHeight}}
BlockID: {{ .ForkBlockID}}
State Hash: {{ .ForkStateHash}}
{{ end }}
```
