package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/local/mayak/internal/remote"
)

func TestUnreachableOnly(t *testing.T) {
	unreachable := fmt.Errorf("%w: websocket: bad handshake", remote.ErrUnreachable)
	written := errors.New("write: broken pipe")
	for _, c := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{unreachable, true},
		{errors.Join(unreachable, unreachable), true},
		{errors.Join(unreachable, written), false},
		{written, false},
	} {
		if got := unreachableOnly(c.err); got != c.want {
			t.Errorf("unreachableOnly(%v) = %t, want %t", c.err, got, c.want)
		}
	}
}
