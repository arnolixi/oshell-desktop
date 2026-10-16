package core

// PublishDownloadedFile publishes a complete, closed staging file on the same
// filesystem without replacing an existing destination. Callers clean up the
// staging path afterwards, whether publication succeeds or fails.
func PublishDownloadedFile(staged, destination string) error {
	return publishDownload(staged, destination, false, false)
}
