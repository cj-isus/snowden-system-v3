package main

// onboarding_dialogs.go — файловые диалоги для onboarding (V2-049/F8).
// Вынесено в отдельный файл, чтобы импорт wails/runtime жил в одном месте.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/onboarding"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Fixed input-domain owners survive a failed acquisition/Close. Never allocate
// a replacement per path or retry an uncertain reader to bypass retirement.
var (
	pinPolicyInput          = filetxn.NewSelectedInputSlot(onboarding.MaxPinPolicyCapsuleSize)
	protectedTransferInput  = filetxn.NewSelectedInputSlot(onboarding.MaxProtectedTransferSize)
	protectedTransferOutput = filetxn.NewSelectedOutputSlot(onboarding.MaxProtectedTransferSize)
)

// Public policy bytes stay in the backend-owned draft; the UI receives a
// preview/token only. The selected file supplies no enrollment authority.
func (a *App) selectPinPolicyFile(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("pin policy: контекст выбора файла недоступен")
	}
	if !profileRequestCurrent(ctx) {
		return nil, context.Canceled
	}
	path, err := wruntime.OpenFileDialog(ctx, wruntime.OpenDialogOptions{Filters: []wruntime.FileFilter{{DisplayName: "Публичный список отзывов (*.json)", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return nil, err // a canceled dialog is not a failed import
	}
	return readSelectedImportFile(ctx, pinPolicyInput, path, onboarding.MaxPinPolicyCapsuleSize)
}

// SaveOnboardingFile — диалог сохранения transport-строки в файл (.snob).
func (a *App) SaveOnboardingFile(passphrase string) (string, error) {
	if err := onboardingPassphraseError(passphrase); err != nil {
		return "", err
	}
	var path string
	err := a.withTransferSource(func(ctx context.Context) error {
		transport, err := a.exportLegacyProfile(ctx, passphrase)
		if err != nil {
			return err
		}
		path, err = a.saveProtectedProfileTransfer(ctx, transport)
		return err
	})
	if err != nil && path != "" {
		return path, fmt.Errorf("onboarding: сохранение не подтверждено; выбранный файл уже мог быть опубликован: %w", err)
	}
	return path, err
}

// Save only an already encrypted, bounded transport string. The selected path
// comes from the owner dialog, never public metadata. Saving does not renew
// delivery/epoch/profile validity; the destination must repeat acceptance.
func (a *App) SaveProtectedProfileTransfer(transport string) (string, error) {
	if err := onboarding.ValidateProtectedTransport(transport); err != nil {
		return "", err
	}
	var path string
	err := a.withTransferSource(func(ctx context.Context) error {
		var inner error
		path, inner = a.saveProtectedProfileTransfer(ctx, transport)
		return inner
	})
	if err != nil && path != "" {
		return path, fmt.Errorf("onboarding: сохранение не подтверждено; выбранный файл уже мог быть опубликован: %w", err)
	}
	return path, err
}

func (a *App) saveProtectedProfileTransfer(ctx context.Context, transport string) (string, error) {
	if !profileRequestCurrent(ctx) {
		return "", context.Canceled
	}
	if err := onboarding.ValidateProtectedTransport(transport); err != nil {
		return "", err
	}
	path, err := wruntime.SaveFileDialog(ctx, wruntime.SaveDialogOptions{
		DefaultFilename: "snowden-onboarding.snob",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Onboarding-бандл (*.snob)", Pattern: "*.snob"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // отмена — не ошибка
	}
	if !profileRequestCurrent(ctx) {
		return "", context.Canceled
	}
	if info, statErr := os.Lstat(path); statErr == nil && !info.Mode().IsRegular() {
		return "", errors.New("onboarding: выберите обычный файл для зашифрованного переноса")
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	}
	outputContext := filetxn.WithSelectedOutputGuard(ctx, func() error {
		if !profileRequestCurrent(ctx) {
			return context.Canceled
		}
		return nil // negative source values only; no App/journal lock or I/O
	})
	publication, err := protectedTransferOutput.WriteSelected(outputContext, path, []byte(transport))
	if err != nil {
		if publication.Published {
			return path, fmt.Errorf("onboarding: файл опубликован по выбранному пути, но завершение сохранения не подтверждено: %w", err)
		}
		if publication.MayHavePublished {
			return path, fmt.Errorf("onboarding: файл мог быть сохранён, завершение записи не подтверждено; проверьте выбранный файл: %w", err)
		}
		return "", fmt.Errorf("onboarding: запись файла: %w", err)
	}
	return path, nil // complete publication/flush/Close and final context gate; no late callback
}

// LoadOnboardingFile — диалог выбора .snob-файла, возвращает transport-строку.
func (a *App) LoadOnboardingFile() (string, error) {
	var transport string
	err := a.withTransferSource(func(ctx context.Context) error {
		var inner error
		transport, inner = a.loadOnboardingFile(ctx)
		return inner
	})
	if err != nil {
		return "", err
	}
	return transport, nil
}

func (a *App) loadOnboardingFile(ctx context.Context) (string, error) {
	if !profileRequestCurrent(ctx) {
		return "", context.Canceled
	}
	path, err := wruntime.OpenFileDialog(ctx, wruntime.OpenDialogOptions{
		Filters: []wruntime.FileFilter{
			{DisplayName: "Onboarding-бандл (*.snob)", Pattern: "*.snob"},
			{DisplayName: "Все файлы", Pattern: "*"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	return readOnboardingTransportContext(ctx, path)
}

// Bound the file read before loading it into memory or invoking the codec.
func readOnboardingTransport(path string) (string, error) {
	return readOnboardingTransportContext(context.Background(), path)
}

func readOnboardingTransportContext(ctx context.Context, path string) (string, error) {
	data, err := readSelectedImportFile(ctx, protectedTransferInput, path, onboarding.MaxProtectedTransferSize)
	if err != nil {
		return "", fmt.Errorf("onboarding: чтение файла: %w", err)
	}
	raw := string(data)
	if strings.TrimSpace(raw) == "" {
		return "", onboarding.ErrBadFormat
	}
	if err := onboarding.ValidateProtectedTransport(raw); err != nil {
		return "", err
	}
	if !profileRequestCurrent(ctx) {
		return "", context.Canceled
	}
	return strings.TrimSpace(raw), nil
}

// Return only exact bytes from a retired original reader. Cancellation gates
// publication before/after synchronous I/O; it does not force-cancel that I/O.
func readSelectedImportFile(ctx context.Context, slot *filetxn.BoundedReadSlot, path string, limit int64) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("onboarding: контекст чтения файла недоступен")
	}
	if !profileRequestCurrent(ctx) {
		return nil, context.Canceled
	}
	raw, err := slot.Read(path, limit)
	if err != nil {
		// The slot replaces an earlier size/open failure with ErrReadClose
		// on uncertain retirement; never classify that as a harmless overflow.
		if errors.Is(err, filetxn.ErrReadTooLarge) {
			return nil, onboarding.ErrTooLarge
		}
		return nil, err
	}
	if !profileRequestCurrent(ctx) {
		return nil, context.Canceled
	}
	if len(raw) == 0 {
		return nil, onboarding.ErrBadFormat
	}
	return raw, nil // no decoding, credentials, trust or enrollment authority here
}
