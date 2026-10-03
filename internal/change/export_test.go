package change

// SetBeforeApplyCommit lets the tests of this folder see the vault as a crash right before
// an apply's commit would leave it.
func SetBeforeApplyCommit(f func()) { beforeApplyCommit = f }

// SetAfterUndoRestore lets the tests of this folder act right after an undo restored its
// paths.
func SetAfterUndoRestore(f func()) { afterUndoRestore = f }
