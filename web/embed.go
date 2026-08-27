// Package web embeds the built frontend (web/dist, produced by `npm run
// build`) so the client binary can serve it without any external files.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
