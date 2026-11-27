package webui

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var rawFS embed.FS

func StaticFS() fs.FS {
	sub, err := fs.Sub(rawFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
