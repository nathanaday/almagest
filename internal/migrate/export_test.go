package migrate

// SetBeforeCommit lets the tests of this folder act right before the migration commits.
func SetBeforeCommit(f func()) { beforeCommit = f }
