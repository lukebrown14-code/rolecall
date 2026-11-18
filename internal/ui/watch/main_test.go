package watch

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/testenv"
)

// TestMain pins the environment (internal/testenv) so a developer's NO_COLOR, TERM, locale
// or $EDITOR never changes a test's outcome; a test of one of those modes sets it itself.
func TestMain(m *testing.M) { testenv.Main(m) }
