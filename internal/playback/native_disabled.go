//go:build !cgo

package playback

import "fmt"

func New(Options) (Engine, error) {
	return nil, fmt.Errorf("embedded video playback requires a CGO-enabled build and libmpv; screenshots remain available")
}
