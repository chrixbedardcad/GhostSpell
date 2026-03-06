package gui

import "embed"

//go:embed frontend/index.html frontend/wizard.html frontend/ghost-icon.png
var frontendFS embed.FS
