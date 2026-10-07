package migrate

// SetBeforeCommit sets the hook that runs right before the migration's commit.
func SetBeforeCommit(f func()) { beforeCommit = f }
