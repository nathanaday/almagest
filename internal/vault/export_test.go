package vault

// SetBeforeRename lets the tests of this folder act inside the window between the sync
// of a temporary file and its rename.
func SetBeforeRename(f func(file string)) { beforeRename = f }
