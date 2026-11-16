//go:build !windows

package filetxn

func newJournalDirectory() journalDirectory { return nil }
