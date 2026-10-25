package queue

import (
	"time"
)

// timeNowUTC is a package-level indirection so tests can pin time
// deterministically.
var timeNowUTC = func() time.Time { return time.Now().UTC() }
