❌ Nodes are on different chains at height {{.Height}}
{{ with .Chains }}{{range .}}
BlockID: {{.BlockID}}
Generator: {{.GeneratorAddress}}
{{end}}{{end}}
