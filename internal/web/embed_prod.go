//go:build !dev

package web

import "embed"

//go:embed static/*
var staticFiles embed.FS
