// Package playback owns decoding and synchronized audio/video. It has no UI or queue.
package playback

import (
	"errors"
	"image"
)

var ErrClosed = errors.New("video decoder is closed")

type Options struct {
	Library string
	// AudioOutput is empty for the system output; "null" is useful in integration tests.
	AudioOutput string
}

type Track struct {
	ID                    int
	Kind, Title, Language string
	Selected              bool
}

type Snapshot struct {
	Generation                        uint64
	Path                              string
	Position, Duration, Volume, Speed float64
	Paused, Idle, Muted, Loaded       bool
	Width, Height                     int
	Tracks                            []Track
	Error                             string
}

type Event struct {
	Snapshot Snapshot
	Ended    bool // Natural EOF only. A stop or replacement never advances the queue.
}

type Frame struct {
	Generation uint64
	Image      *image.NRGBA
}

type Command struct {
	Args       []string
	Generation uint64 // A load's identity; repeated loads of the same file remain distinct.
}

type Engine interface {
	Send(Command) error
	Events() <-chan Event
	Frames() <-chan Frame
	Resize(width, height int)
	Close() error
}
