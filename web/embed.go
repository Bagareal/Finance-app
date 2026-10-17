// Package web incorpora il frontend statico nel binario.
package web

import "embed"

//go:embed index.html app.js i18n.js style.css manifest.webmanifest sw.js icon-192.png icon-512.png vendor
var FS embed.FS
