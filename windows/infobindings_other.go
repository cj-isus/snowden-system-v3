//go:build !windows

package main

// infobindings_other.go — заглушки netwatch/transportpref вне Windows.
// Продукт — Windows-only; заглушки существуют, чтобы `go test ./...` на
// другой машине агента компилировался и honest-failed по фактам, а не по
// отсутствию файлов.

// adapterNetClass — вне Windows класс сети неизвестен.
func adapterNetClass(alias string) string { return "" }

// adapterNetClassByIndex — вне Windows класс сети неизвестен. Windows-сборщик
// берёт факт мгновенным системным вызовом (GetIfEntry2Ex); заглушка сохраняет
// «факт недоступен» без сети.
func adapterNetClassByIndex(idx int) string { return "" }
