//go:build !unix

package main

import "os"

// Store.Append takes no lock on this platform either, so there is no writer to
// wait for.
func lockShared(*os.File) error { return nil }
