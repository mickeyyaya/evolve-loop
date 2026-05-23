package detectcli

import "os"

// Split out so test files can override osGetenvImpl in TestMain if needed.
var osGetenvImpl = os.Getenv
