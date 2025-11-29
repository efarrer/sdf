package internal

import (
	"io"

	"github.com/ejoffe/spr/config"
)

type Gitbase struct {
	Config  *config.Config
	Rootdir string
	Stderr  io.Writer
}
