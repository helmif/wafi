package filters

import (
	"os"
	"testing"
	"time"
)

// TestMain pins logNow to a fixed point in time so golden files produced by
// the git-log filter remain byte-identical on every test run, regardless of
// when the tests are executed. The anchor is 5 days after the date our phase
// commits were made (2026-04-20), so all fixtures show "5 days ago".
func TestMain(m *testing.M) {
	logNow = func() time.Time {
		return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	}
	os.Exit(m.Run())
}
