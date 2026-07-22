package modbus

import "time"

// timeoutForTests is the connect/context timeout used throughout the
// suite: long enough to be immune to CI scheduling jitter, short enough
// that a genuinely hung test still fails fast.
func timeoutForTests() time.Duration { return 2 * time.Second }
