package main

import (
	"context"
	"errors"
	"strings"

	"github.com/snowden-system/windows/backend/core"
)

// Presentation only. Call before taking the App value mutex; keep the original
// error and cleanup classification for lifecycle control and returned outcomes.
// Never format an unknown error, stage, endpoint or channel ID into this text.
func lifecycleFailureText(stage string, cause error, cleanupUnconfirmed bool) string {
	label := lifecycleFailureStage(stage)
	var detail string
	switch {
	case cleanupUnconfirmed:
		detail = "Возврат ресурсов исходной операции не подтверждён. Повторное подключение требует завершённой очистки; готовность защиты не подтверждена."
	case errors.Is(cause, errInstalledImageChanged):
		detail = "Файлы приложения могли измениться. Закройте и заново запустите приложение; при ошибке запуска восстановите установку."
	case errors.Is(cause, core.ErrProxyOwnershipConflict):
		detail = "Системные настройки прокси изменились извне и не переписывались. Автоподключение остановлено; проверьте настройки и явно запустите VPN заново."
	case errors.Is(cause, core.ErrProxyObservationUnavailable):
		detail = "Состояние применённого системного прокси не удалось подтвердить. Автоподключение приостановлено; проверьте локальные настройки и результат остановки. Смена канала эту причину не устраняет."
	case errors.Is(cause, core.ErrProxyWriterBusy):
		detail = "Хранилище управления системным прокси занято другой операцией. Дождитесь её завершения и повторите запуск; смена канала не устраняет эту причину."
	case errors.Is(cause, core.ErrProxyStageConflict):
		detail = "Подготовленные данные прокси не прошли сверку. Операция остановлена; автоматический сброс не выполняется."
	case errors.Is(cause, core.ErrProxyStageUnavailable):
		detail = "Подготовка или чтение зашифрованной копии данных прокси не подтверждены. Новое применение остановлено; автоматический сброс не выполняется."
	case errors.Is(cause, core.ErrProxyAnchorMigrationRequired):
		detail = "Сохранённые данные прокси созданы без новой контрольной записи. Требуется отдельный перенос; автоматический перенос и новое применение остановлены."
	case errors.Is(cause, core.ErrProxyAnchorOldFile):
		detail = "Прочитана старая версия файла управления прокси. Публикация новой версии не подтверждена; сведения о прерванном сохранении оставлены для восстановления. Новое применение остановлено."
	case errors.Is(cause, core.ErrProxyAnchorMissingInitial):
		detail = "Первый файл управления прокси не найден при незавершённой контрольной записи. Данные не сбрасываются автоматически; новое применение остановлено."
	case errors.Is(cause, core.ErrProxyAnchorPending):
		detail = "Предыдущее сохранение данных управления прокси не завершено. Автоматическое восстановление пока недоступно; новое применение остановлено."
	case errors.Is(cause, core.ErrProxyAnchorUnavailable):
		detail = "Файл управления прокси и независимая контрольная запись не согласованы или не подтверждены. Новое применение остановлено; эти данные не сбрасываются автоматически."
	case errors.Is(cause, core.ErrProxyReceiptSubjectChanged):
		detail = "Сохранённые данные управления прокси изменились или не прошли повторную сверку. Новое применение остановлено; автоматический сброс этих данных недоступен."
	case errors.Is(cause, core.ErrProxyPriorOwnerRunning):
		detail = "Предыдущий процесс управления прокси ещё работал во время проверки. Завершите подключение в том экземпляре приложения и повторите запуск."
	case errors.Is(cause, core.ErrProxyPriorOwnerChanged):
		detail = "Идентификатор процесса из квитанции уже принадлежит другому процессу. Это не разрешает восстановление прокси; новое применение остановлено."
	case errors.Is(cause, core.ErrProxyPriorOwnerUnresolved):
		detail = "Состояние процесса из незавершённой квитанции прокси не удалось подтвердить. Его завершение не предполагается; новое применение остановлено."
	case errors.Is(cause, core.ErrProxyRecoveryRequired):
		detail = "Предыдущий процесс завершился. Безопасное восстановление сохранённых настроек прокси пока недоступно; новое применение остановлено."
	case errors.Is(cause, core.ErrProxyReceiptUnavailable):
		detail = "Состояние сохранённой квитанции управления прокси не подтверждено. Новое применение остановлено; автоматическое восстановление пока недоступно."
	case trustFailureCode(cause) != "":
		detail = trustFailureDetail(cause)
	case errors.Is(cause, errSessionSuperseded), errors.Is(cause, core.ErrPreflightScope):
		detail = "Результат не подтверждён для текущей операции или сети. Требуется свежая проверка текущего состояния."
	case errors.Is(cause, context.Canceled):
		detail = "Операция отменена. Её отмена не подтверждает завершение очистки или готовность защиты."
	case errors.Is(cause, context.DeadlineExceeded):
		detail = "Время ожидания операции истекло. Это не подтверждает завершение очистки или причину сетевого сбоя."
	case errors.Is(cause, core.ErrTLSUntrusted):
		detail = "Доверие к сертификату сервера не подтверждено. Проверьте действующий профиль и сертификат; проверку TLS отключать нельзя."
	case errors.Is(cause, core.ErrTLSHostMismatch):
		detail = "Сертификат сервера не соответствует ожидаемому имени. Проверьте профиль и настройки сервера."
	case errors.Is(cause, core.ErrTLSInvalid):
		detail = "Сертификат сервера недействителен. Требуется действующий сертификат и согласованный профиль."
	default:
		detail = "Операция не завершена; причина не классифицирована. Подключение и готовность защиты не подтверждены."
	}
	return label + ": " + detail
}

func lifecycleFailureStage(stage string) string {
	switch stage {
	case "контроль туннеля не подтверждён":
		return "Контроль туннеля"
	case "наблюдатели после сна не восстановлены", "непрерывность после сна требует нового запуска":
		return "Восстановление после сна"
	case "WFP: барьер не подтверждён до подготовки конфига":
		return "Подготовка системной защиты"
	case "сетевой наблюдатель или DNS-обработчик не завершён", "обработчик сетевого DNS завершился без подтверждения":
		return "Сетевые наблюдатели"
	case "смена сетевого DNS-контекста не подтверждена":
		return "Смена сетевого DNS-контекста"
	case "рендер конфига":
		return "Подготовка конфигурации"
	case "владение или очистка процессов не подтверждены":
		return "Проверка владения ресурсами"
	case "запуск", "запуск: результат уже не принадлежит текущей сессии/сети":
		return "Запуск VPN"
	case "результат переключения утратил привязку к текущей сессии/сети":
		return "Переключение канала"
	case "rollback утратил привязку к текущей сессии/сети":
		return "Восстановление предыдущего канала"
	case "сохранённая сессия утратила привязку к текущей операции/сети":
		return "Проверка сохранённой сессии"
	case "probe: возврат ресурсов не подтверждён", "probe: подтверждён direct leak", "probe: защита TUN/WFP не подтверждена", "probe: egress-адреса не соответствуют сравнению":
		return "Проверка защищённого пути"
	case "операция жизненного цикла отменена":
		return "Операция VPN"
	case "остановка":
		return "Остановка VPN"
	case "перезапуск":
		return "Перезапуск приложения"
	default:
		if strings.HasPrefix(stage, "переключение на ") {
			return "Переключение канала" // caller label is deliberately not copied
		}
		return "Операция VPN"
	}
}
