//+build tools

// Package tools tracks dependencies for tools that used in the build process.
// See https://github.com/golang/go/wiki/Modules
package hack

import (
	_ "gotest.tools/gotestsum"
	_ "github.com/ahmetb/govvv"
)
