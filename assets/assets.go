package assets

import _ "embed"

//go:embed GhostSpell_icon_512.png
var AppIcon512 []byte

//go:embed GhostSpell_icon_16.png
var AppIcon16 []byte

//go:embed GhostSpell_icon_32.png
var AppIcon32 []byte

//go:embed GhostSpell_tray_64.png
var TrayIcon64 []byte

//go:embed GhostSpell_tray_64_macOS.png
var TrayIconMacOS []byte

//go:embed ghostspell.ico
var AppIconICO []byte
