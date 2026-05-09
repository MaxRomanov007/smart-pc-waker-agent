package updater

import "os"

// readFile reads a small file into a string. Used to inspect /proc entries.
func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
