package player

import (
	"github.com/codemodify/paintengine2d"
	"github.com/codemodify/uitoolkit/style"
)

type queueLook struct {
	style.LookAndFeel
	background paintengine2d.Color
	skin       string
}

func (l *queueLook) ViewBackground(style.ControlState) paintengine2d.Color { return l.background }
