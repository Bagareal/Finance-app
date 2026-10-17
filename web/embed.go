// Package web incorpora il frontend statico nel binario.
package web

import "embed"

//go:embed index.html app.js i18n.js style.css manifest.webmanifest sw.js icon.svg app-icon-192.png app-icon-512.png app-icon-maskable-512.png apple-touch-icon.png vendor
var FS embed.FS
