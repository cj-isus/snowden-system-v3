package main

// secrettests.go — живые тесты для каждого вида секрета (кнопка «Тест»
// в карточке секрета). Правила:
//   - значение секрета не покидает процесс и не попадает в логи/UI:
//     сверка с сервером — только по SHA256-префиксам;
//   - каждый шаг с человекочитаемым результатом (факты, не гипотезы);
//   - «skipped» — честный исход, когда тест неприменим (не ошибка).
//
// Ожидание для vps-ssh-key (fingerprint установленного на сервере ключа) —
// в gitignored-файле .tmp/expectations.v1.json, не в коде: ключ ротируется.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
	"golang.org/x/crypto/ssh"
)

// SecretTestReport — результат теста одного секрета (JSON в wailsjs).
type SecretTestReport struct {
	OK    bool            `json:"ok"`
	Steps []ProbeStepView `json:"steps"`
}

// testServerHost — резервный origin канала A: используется ТОЛЬКО если
// дескрипторы не читаются (повреждённая сборка), чтобы живой тест оставался
// честным фактом о существующем сервере, а не молчал.
const testServerHost = "203.0.113.10"

// testEndpoints — контакты живых тестов из дескрипторов (FR-002: данные,
// не хардкод). SSH (сверка хешей/вход ключом) — на origin-сервер VLESS-канала
// (deployed-конфиг живёт там); UDP-достижимость — на origin:port hysteria2-
// канала. Включённый канал важнее статуса: тест существует, чтобы статус
// уточнить.
func testEndpoints() (sshHost, hy2Host, hy2Port string) {
	sshHost, hy2Host, hy2Port = testServerHost, testServerHost, "8444"
	channels, err := render.LoadDescriptors()
	if err != nil {
		return
	}
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		if ch.Protocol == "vless" {
			sshHost = ch.OriginServer
		}
		if ch.Protocol == "hysteria2" {
			hy2Host = ch.OriginServer
			hy2Port = strconv.Itoa(int(ch.Port))
		}
	}
	return
}

const (
	testSSHUser     = "root"
	testDialTimeout = 10 * time.Second
	testHTTPTimeout = 15 * time.Second
)

// pinnedServerHostKeys — публичные host-ключи сервера (ssh-keyscan + живой
// probe 2026-09-09). Публичный материал: держим в коде, чтобы SSH-тест не
// зависел от known_hosts (домашний каталог с кириллицей его не сохраняет).
// Сервер подписывает разными алгоритмами (ed25519/ecdsa) — сверяем по списку.
var pinnedServerHostKeys = []string{
	"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDoLPNNm8+k6f16UONEGFjgtYFB98XLksKNccuPggALQ",
	"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBKBSin40JEEc11B9K74KxG5wipW1tBmzbKu7KE/0x/zcBkBO4uzC8L6V/656XB99On9pfUqvAKOoL0yzQAr1tvY=",
}

// pinnedHostKeyCallback — допуск только ключей из закреплённого списка
// (защита от MITM; отсутствие совпадения = ошибка, не «принять любой»).
func pinnedHostKeyCallback() ssh.HostKeyCallback {
	pinned := map[string]bool{}
	for _, line := range pinnedServerHostKeys {
		if pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line)); err == nil {
			pinned[base64.StdEncoding.EncodeToString(pk.Marshal())] = true
		}
	}
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if pinned[base64.StdEncoding.EncodeToString(key.Marshal())] {
			return nil
		}
		return fmt.Errorf("ключ сервера не совпадает с закреплённым: возможен MITM, подключение прервано")
	}
}

// keyExpectation — ожидания для теста vps-ssh-key.
type keyExpectation struct {
	PrivateKeyFingerprint string `json:"private_key_fingerprint"`
}

func expectationsPaths() []string {
	paths := []string{}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".tmp", "expectations.v1.json"))
	}
	return append(paths, filepath.Join(".tmp", "expectations.v1.json"))
}

func loadKeyExpectation() (keyExpectation, bool) {
	for _, p := range expectationsPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var e keyExpectation
		if json.Unmarshal(b, &e) == nil && e.PrivateKeyFingerprint != "" {
			return e, true
		}
	}
	return keyExpectation{}, false
}

// ---------- диспетчер по виду секрета ----------

// runSecretTest — точка входа из app.go. Возвращает отчёт; карточку обновляет
// вызывающий (статус проверки живой тест не меняет — он про сеть, не про формат).
func (a *App) runSecretTest(id string) (SecretTestReport, error) {
	meta, err := a.refetchMeta(id)
	if err != nil {
		return SecretTestReport{}, err
	}
	if meta.StoredValue == "" {
		return SecretTestReport{OK: false, Steps: []ProbeStepView{{
			Name: "значение", Status: "skipped",
			Detail: "значение не задано — тестировать нечего (сначала «Задать значение»)",
		}}}, nil
	}
	value, err := a.vault.Value(id)
	if err != nil {
		return SecretTestReport{}, fmt.Errorf("значение недоступно: %w", err)
	}

	switch secretvault.Kind(meta.Kind) {
	case secretvault.KindVlessUUID:
		return testChannelSecret(value, "uuid", meta.Fingerprint,
			"UUID VLESS", "users[0].uuid в vless-инбаунде"), nil
	case secretvault.KindHy2Password:
		return testChannelSecret(value, "hy2", meta.Fingerprint,
			"пароль HY2", "users[0].password в hysteria2-инбаунде"), nil
	case secretvault.KindHy2ObfsPasword:
		return testChannelSecret(value, "obfs", meta.Fingerprint,
			"пароль obfs (salamander)", "obfs.password в hysteria2-инбаунде"), nil
	case secretvault.KindVpsSSHKey:
		return testSSHKey(value, meta.Fingerprint), nil
	case secretvault.KindCfApiToken:
		return testCfToken(value, meta.Fingerprint), nil
	default:
		return SecretTestReport{OK: false, Steps: []ProbeStepView{{
			Name: "живой тест", Status: "skipped",
			Detail: "для произвольных значений живой тест не определён — доступна только проверка формата",
		}}}, nil
	}
}

// ---------- канальные секреты: сверка хешей с сервером ----------

// srvHashScript выполняется НА СЕРВЕРЕ (python3 по SSH). Наружу идут только
// SHA256-префиксы (16 hex) значений из deployed-конфига. Пути к секретам в
// конфиге: vless → users[0].uuid; hysteria2 → users[0].password, obfs.password
// (проверено на сервере 2026-09-09: именно эта структура даёт известные хеши).
const srvHashScript = `import json, hashlib
o = {}
for i in json.load(open('/etc/sing-box/config.json')).get('inbounds', []):
    t = i.get('type')
    if t == 'vless':
        u = (i.get('users') or [{}])[0].get('uuid') or ''
        o['uuid'] = hashlib.sha256(u.encode()).hexdigest()[:16]
    if t == 'hysteria2':
        pw = (i.get('users') or [{}])[0].get('password') or ''
        ob = (i.get('obfs') or {}).get('password') or ''
        o['hy2'] = hashlib.sha256(pw.encode()).hexdigest()[:16]
        o['obfs'] = hashlib.sha256(ob.encode()).hexdigest()[:16]
print(json.dumps(o))
`

// srvHashes — результат серверного скрипта.
type srvHashes struct {
	UUID string `json:"uuid"`
	HY2  string `json:"hy2"`
	Obfs string `json:"obfs"`
}

// testChannelSecret — общий сценарий для uuid/hy2/obfs:
//  1. формат значения (локально, тот же валидатор, что при сохранении);
//  2. TCP 22 сервера достижим;
//  3. SSH-сессия: сервер сам считает хеши deployed-конфига, сравниваем префиксы;
//  4. итог по конкретному секрету.
func testChannelSecret(value, key, localFp, human, where string) SecretTestReport {
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	fail := func(step, detail string) SecretTestReport {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: step, Status: "fail", Detail: detail})
		return rep
	}

	kind := map[string]secretvault.Kind{
		"uuid": secretvault.KindVlessUUID,
		"hy2":  secretvault.KindHy2Password,
		"obfs": secretvault.KindHy2ObfsPasword,
	}[key]
	if err := secretvault.ValidateKind(kind, value); err != nil {
		return fail("формат значения", err.Error())
	}
	rep.Steps = append(rep.Steps, ProbeStepView{
		Name: "формат значения", Status: "pass",
		Detail: fmt.Sprintf("%s: формат корректен (fingerprint %s)", human, localFp),
	})

	addr := addrForSSH()
	conn, err := net.DialTimeout("tcp", addr, testDialTimeout)
	if err != nil {
		return fail("SSH-транспорт", fmt.Sprintf("TCP %s: %v", addr, err))
	}
	defer conn.Close()
	rep.Steps = append(rep.Steps, ProbeStepView{
		Name: "SSH-транспорт", Status: "pass", Detail: "TCP 22 открыт",
	})

	hashes, detail, ok := runSrvHashScript(conn)
	if !ok {
		return fail("серверные хеши", detail)
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "серверные хеши", Status: "pass", Detail: detail})

	got := map[string]string{"uuid": hashes.UUID, "hy2": hashes.HY2, "obfs": hashes.Obfs}[key]
	switch {
	case got == "":
		return fail("итог", "сервер не вернул хеш для "+where)
	case got == localFp:
		rep.Steps = append(rep.Steps, ProbeStepView{
			Name: "итог", Status: "pass",
			Detail: fmt.Sprintf("хеш на сервере (%s) совпадает с локальным значением — сервер работает с этим %s", got, human),
		})
	default:
		return fail("итог", fmt.Sprintf("РАСХОЖДЕНИЕ: на сервере %s, локально %s — сервер работает с другим значением", got, localFp))
	}
	rep.OK = true
	return rep
}

// runSrvHashScript исполняет srvHashScript в SSH-сессии (скрипт — через stdin,
// `python3`). Значения секретов через сеть не ходят — только 16-hex префиксы.
func runSrvHashScript(conn net.Conn) (srvHashes, string, bool) {
	signers, err := keySigners()
	if err != nil {
		return srvHashes{}, err.Error(), false
	}
	cfg := &ssh.ClientConfig{
		User:            testSSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		HostKeyCallback: pinnedHostKeyCallback(),
		Timeout:         testDialTimeout,
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addrForSSH(), cfg)
	if err != nil {
		return srvHashes{}, "SSH-рукопожатие/аутентификация: " + err.Error(), false
	}
	defer c.Close()
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return srvHashes{}, "SSH-сессия: " + err.Error(), false
	}
	defer sess.Close()
	sess.Stdin = strings.NewReader(srvHashScript)
	out, err := sess.CombinedOutput("python3")
	if err != nil {
		return srvHashes{}, "серверный скрипт: " + err.Error() + "; вывод: " + truncateStr(out, 200), false
	}
	var hashes srvHashes
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &hashes); err != nil {
		return srvHashes{}, "не JSON в ответе сервера: " + truncateStr(out, 160), false
	}
	return hashes, fmt.Sprintf("сервер: uuid=%s hy2=%s obfs=%s", hashes.UUID, hashes.HY2, hashes.Obfs), true
}

func addrForSSH() string {
	sshHost, _, _ := testEndpoints()
	return net.JoinHostPort(sshHost, "22")
}

// keySigners — подписанты из ~/.ssh/vps_key (путь — владелецский, не секрет).
func keySigners() ([]ssh.Signer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("домашний каталог: %w", err)
	}
	key, err := os.ReadFile(filepath.Join(home, ".ssh", "vps_key"))
	if err != nil {
		return nil, fmt.Errorf("файл ~/.ssh/vps_key не прочитан (%v): доступ по ключу не настроен", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("~/.ssh/vps_key: %w", err)
	}
	return []ssh.Signer{signer}, nil
}

func truncateStr(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}

// ---------- vps-ssh-key: реальный вход ключом ИЗ ХРАНИЛИЩА ----------

// testSSHKey проверяет само значение из хранилища: парсим PEM, сверяем с
// ожиданием (если файл ожиданий есть), затем делаем настоящий SSH-вход этим
// ключом и исполняем echo. Значение не печатается и не логируется.
func testSSHKey(value, localFp string) SecretTestReport {
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	fail := func(step, detail string) SecretTestReport {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: step, Status: "fail", Detail: detail})
		return rep
	}

	signer, err := ssh.ParsePrivateKey([]byte(value))
	if err != nil {
		return fail("разбор ключа", "PEM не разобран: "+err.Error())
	}
	rep.Steps = append(rep.Steps, ProbeStepView{
		Name: "разбор ключа", Status: "pass",
		Detail: "приватный ключ корректен (fingerprint " + localFp + ")",
	})

	if exp, ok := loadKeyExpectation(); ok {
		if exp.PrivateKeyFingerprint == localFp {
			rep.Steps = append(rep.Steps, ProbeStepView{
				Name: "сверка с ожиданием", Status: "pass",
				Detail: "fingerprint совпадает с ожидаемым (этот ключ установлен на сервере)",
			})
		} else {
			return fail("сверка с ожиданием",
				fmt.Sprintf("fingerprint %s не совпадает с ожидаемым %s — в хранилище не тот ключ", localFp, exp.PrivateKeyFingerprint))
		}
	} else {
		rep.Steps = append(rep.Steps, ProbeStepView{
			Name: "сверка с ожиданием", Status: "skipped",
			Detail: "файл ожиданий .tmp/expectations.v1.json отсутствует — сверка пропущена, живой вход продолжается",
		})
	}

	addr := addrForSSH()
	dialer := &net.Dialer{Timeout: testDialTimeout}
	conn, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return fail("SSH-транспорт", fmt.Sprintf("TCP %s: %v", addr, err))
	}
	defer conn.Close()
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "SSH-транспорт", Status: "pass", Detail: "TCP 22 открыт"})

	cfg := &ssh.ClientConfig{
		User:            testSSHUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: pinnedHostKeyCallback(),
		Timeout:         testDialTimeout,
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addrForSSH(), cfg)
	if err != nil {
		return fail("аутентификация ключом", "сервер отклонил ключ из хранилища: "+err.Error())
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return fail("аутентификация ключом", "SSH-сессия: "+err.Error())
	}
	defer sess.Close()
	type cmdResult struct {
		out []byte
		err error
	}
	resCh := make(chan cmdResult, 1)
	go func() {
		o, e := sess.CombinedOutput("echo ok")
		resCh <- cmdResult{out: o, err: e}
	}()
	select {
	case res := <-resCh:
		if res.err != nil || strings.TrimSpace(string(res.out)) != "ok" {
			return fail("аутентификация ключом", "вход выполнен, команда не подтверждена: "+errStr(res.err))
		}
	case <-time.After(10 * time.Second):
		return fail("аутентификация ключом", "сервер не ответил на команду за 10 с")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{
		Name: "аутентификация ключом", Status: "pass",
		Detail: "сервер принял ключ из хранилища, команда выполнена — ключ живой и авторизован",
	})
	rep.OK = true
	return rep
}

func errStr(e error) string {
	if e == nil {
		return "пустой ответ"
	}
	return e.Error()
}

// ---------- cf-api-token: живая проверка через API Cloudflare ----------

// testCfToken — реальный вызов /user/tokens/verify. Значение уходит только
// в заголовок Authorization к api.cloudflare.com (это единственное назначение
// токена); в логи/UI не попадает.
func testCfToken(value, localFp string) SecretTestReport {
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	fail := func(step, detail string) SecretTestReport {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: step, Status: "fail", Detail: detail})
		return rep
	}

	if err := secretvault.ValidateKind(secretvault.KindCfApiToken, value); err != nil {
		return fail("формат значения", err.Error())
	}
	rep.Steps = append(rep.Steps, ProbeStepView{
		Name: "формат значения", Status: "pass",
		Detail: "длина корректна (fingerprint " + localFp + ")",
	})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
	if err != nil {
		return fail("HTTP-запрос", err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+value)
	client := &http.Client{Timeout: testHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fail("HTTP-запрос", "сеть недоступна: "+err.Error())
	}
	defer resp.Body.Close()

	var body struct {
		Success bool `json:"success"`
		Result  *struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"result"`
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fail("ответ API", fmt.Sprintf("HTTP %d, тело не разобрано: %v", resp.StatusCode, err))
	}
	switch {
	case body.Success && body.Result != nil && body.Result.Status == "active":
		rep.Steps = append(rep.Steps, ProbeStepView{
			Name: "ответ API", Status: "pass",
			Detail: "Cloudflare: токен ACTIVE (id " + body.Result.ID + ")",
		})
		rep.OK = true
	case body.Success:
		return fail("ответ API", fmt.Sprintf("Cloudflare: токен в состоянии %q — не активен", body.Result.Status))
	default:
		msg := "отказ без деталей"
		if len(body.Errors) > 0 {
			msg = fmt.Sprintf("Cloudflare: код %d — %s", body.Errors[0].Code, body.Errors[0].Message)
		}
		return fail("ответ API", msg+" (токен отозван/неверен/истёк)")
	}
	return rep
}
