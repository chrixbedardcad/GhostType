package gui

import "embed"

//go:embed frontend/index.html frontend/wizard.html frontend/indicator.html frontend/ghost-icon.png frontend/ghostspell-ghost.png frontend/ghostAI.png
var frontendFS embed.FS
