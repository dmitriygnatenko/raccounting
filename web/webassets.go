// Package web holds only the embedded frontend — go:embed patterns can't reach outside the
// directory of the file that declares them, so this lives next to index.html/css/js/assets,
// separate from the actual binary entrypoint in cmd/raccounting.
package web

import "embed"

// WebFiles is the built frontend, embedded into the binary.
//
//go:embed index.html css js assets
var WebFiles embed.FS
