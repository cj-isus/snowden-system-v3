//go:build !windows

package filetxn

func newJournalTargets() journalTargets { return nil }
