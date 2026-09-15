package main

// transportpref.go — авто-выбор транспорта по классу сети (V2-054).
//
// Поле 2026 из журнала владельца: на мобильном CGNAT CDN-путь (vless+ws
// через CF) тонет волнами (V2-026/040), а QUIC-origin HY2 в те же минуты
// жив; на домашнем Wi-Fi оба пути живы, но CF-WS дешевле и не тратит
// «дорогой» UDP. Класс сети — факт ОС (MediaType адаптера маршрута по
// умолчанию), не эвристика.
//
// Политика МИНИМАЛЬНАЯ и probe-безопасная:
//   - решается только DEFAULT (какой канал стартует первым при Start без
//     явного выбора); failover-on-start (V2-040) в любом случае переберёт
//     остальные validated каналы, если первый не пройдёт probe;
//   - НИКАКИХ вмешательств в работающий туннель и в SelectChannel:
//     явный выбор владельца и авто-failover идут прежними путями;
//   - канал не из списка предпочтений любого класса = дефолт pickDefault
//     (первый live-verified из дескрипторов) — поведение без изменений;
//   - «нет класса» (факт недоступен) = нет предпочтения — прежнее поведение.
//
// Порядок предпочтений (validated каналы, слева лучше):
//   mobile  → hysteria2 (QUIC origin: brutal CC + port hopping живут лучше
//             CGNAT), затем остальное в обычном порядке;
//   wifi / ethernet → vless+ws (CF-CDN) первым, затем hysteria2, затем
//             остальное — CF-путь на фиксированных сетях дешевле UDP.

import (
	"sort"

	"github.com/snowden-system/windows/backend/render"
)

// preferredOrder — порядок каналов для класса сети. Идентификаторы берутся
// из фактических дескрипторов (протокол+транспорт), не наоборот: если
// владелец перееименует канал, порядок молча перестанет матчить — и это
// честнее хардкода ID. Матчим по transport-строке дескриптора.
var preferredOrderByClass = map[string][]string{
	"mobile":   {"hysteria2+quic", "vless+tcp-reality"},
	"wifi":     {"vless+ws", "hysteria2+quic", "vless+tcp-reality"},
	"ethernet": {"vless+ws", "hysteria2+quic", "vless+tcp-reality"},
}

// transportKey — идентичность транспорта дескриптора (protocol+transport),
// формат как в ChannelView.Transport (UI-карточки каналов).
func transportKey(ch render.ChannelDescriptor) string {
	return ch.Protocol + "+" + ch.Transport
}

// pickDefaultChannel — default для Start: если известен класс текущей сети
// и среди enabled live-verified каналов есть совпадающий с предпочтением
// класса — вернуть лучший такой канал. Иначе "" (= прежний pickDefault).
// Чистая функция от (descriptors, class) — покрыта юнит-тестами.
func pickDefaultChannel(descriptors []render.ChannelDescriptor, netClass string) string {
	order, ok := preferredOrderByClass[netClass]
	if !ok {
		return ""
	}
	// Кандидаты: enabled live-verified (тот же гейт, что у селектора).
	type cand struct {
		id    string
		rank  int // позиция в порядке предпочтений; больше = хуже
		index int // позиция в дескрипторах (тай-брейк стабильности)
	}
	var cands []cand
	for i, ch := range descriptors {
		if !ch.Enabled || ch.ValidationStatus != "live-verified" {
			continue
		}
		for r, key := range order {
			if transportKey(ch) == key {
				cands = append(cands, cand{id: ch.ID, rank: r, index: i})
				break
			}
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		return cands[i].index < cands[j].index
	})
	return cands[0].id
}

// currentNetClass — класс сети сейчас (факт ОС; «» = неизвестно). Класс
// берётся из мгновенного факта MIB (netFingerprintOS → adapterNetClassByIndex):
// вызов живёт на пути startCore — PowerShell-спавн здесь добавлял бы секунды
// к каждому запуску. Отказ сбора fingerprint = нет класса (прежнее поведение).
func currentNetClass() string {
	fp := netFingerprintOS()
	if !fp.present() {
		return ""
	}
	return netClassOf(fp)
}
