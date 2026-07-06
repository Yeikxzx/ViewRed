package main

import "embed"

//go:embed all:webdist
var webDistFS embed.FS

//go:embed all:migrations
var migrationsFS embed.FS
