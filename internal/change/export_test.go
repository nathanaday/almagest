package change

// SetBeforeApplyCommit lets the tests of this folder see the vault as a crash right before
// an apply's commit would leave it.
func SetBeforeApplyCommit(f func()) { beforeApplyCommit = f }
