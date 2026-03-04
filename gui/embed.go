package gui

import "embed"

//go:embed frontend/index.html frontend/wizard.html
var frontendFS embed.FS
