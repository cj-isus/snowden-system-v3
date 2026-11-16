//go:build !windows

package filetxn

func newJournalWriterLock(string) journalWriterLock { return nil }
func newJournalReaderLock(string) journalWriterLock { return nil }
