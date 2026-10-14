package instance

import "runtime"

func arch() string { return runtime.GOARCH }
