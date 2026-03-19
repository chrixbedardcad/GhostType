package gui

import "embed"

//go:embed frontend/index.html frontend/wizard.html frontend/indicator.html frontend/update.html frontend/result.html frontend/ghost-icon.png frontend/ghostspell-ghost.png frontend/ghostAI.png all:frontend/dist
var frontendFS embed.FS
