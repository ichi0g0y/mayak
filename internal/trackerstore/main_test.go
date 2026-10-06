package trackerstore

import (
	"os"
	"testing"

	"github.com/local/mayak/internal/testenv"
)

// The tests use a temporary config folder, not this PC's MAYAK data.
func TestMain(m *testing.M) { os.Exit(testenv.Run(m)) }
