//go:build !unix

package memory

import "os"

// No advisory locking available on this platform. Appends of a single short
// record are still effectively atomic under O_APPEND; concurrent writers with
// very long bodies are the only exposure.
func lockFile(f *os.File) (unlock func(), err error) {
	return func() {}, nil
}
