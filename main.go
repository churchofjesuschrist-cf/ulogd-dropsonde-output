package main

// This main package exists only for the purpose of importing the interesting packages.
// Because this is built with cgo and -buildmode=c-shared, the main func is not included
// in the output object file. As well, the imported packages are only imported for the
// (cgo) side-effects.

import _ "github.com/lds-cf/ulogd-dropsonde-output/plugin"

func main() {}
