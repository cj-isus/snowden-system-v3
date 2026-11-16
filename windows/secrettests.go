package main

// Production implementation of the explicit GUI secret diagnostic, not Go tests.
// Credentials are used only for their purpose (SSH signing / HTTPS Authorization).
// Results do not echo raw transport/server errors, output or credential strings.
// This diagnostic is not protected-egress, server-configuration or trust acceptance.
//
// Ожидание для vps-ssh-key (fingerprint установленного на сервере ключа) —
// в gitignored-файле .tmp/expectations.v1.json, не в коде: ключ ротируется.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

// ---------- диспетчер по виду секрета ----------

// The original encrypted source supplies both metadata and values. No App lock
// or vault/journal lock crosses a network call. Rechecking does not undo an
// already-sent credential, nor make the observation atomic with a later mutation.
func runSecretTest(task *manualDiagnosticTask, vault *secretvault.Manager, id string) (SecretTestReport, secretvault.Meta, error) {
	if err := task.currentError(); err != nil {
		return SecretTestReport{}, secretvault.Meta{}, err
	}
	snapshot, err := vault.PrepareImportSnapshot(nil)
	if err != nil {
		return SecretTestReport{}, secretvault.Meta{}, errSecretTestUnavailable
	}
	meta, err := snapshot.MetaByID(id)
	if err != nil {
		return SecretTestReport{}, secretvault.Meta{}, errSecretTestUnavailable
	}
	value := ""
	if meta.StoredValue != "" {
		slot, err := snapshot.ExportReference(id, meta.Kind)
		if err != nil {
			return SecretTestReport{}, secretvault.Meta{}, errSecretTestUnavailable
		}
		value = slot.Value
		meta.Fingerprint = secretvault.Fingerprint(value)
	}
	if err := vault.WithPreparedSnapshot(snapshot, task.currentError); err != nil {
		return SecretTestReport{}, secretvault.Meta{}, errSecretTestUnavailable
	}
	var rep SecretTestReport
	if meta.StoredValue == "" {
		rep.Steps = []ProbeStepView{{Name: "значение", Status: "skipped", Detail: "значение не задано — проверка не выполнялась"}}
	} else {
		switch secretvault.Kind(meta.Kind) {
		case secretvault.KindVlessUUID:
			rep = testChannelSecret(task, snapshot, value, "uuid", meta.Fingerprint, "UUID VLESS")
		case secretvault.KindHy2Password:
			rep = testChannelSecret(task, snapshot, value, "hy2", meta.Fingerprint, "пароль HY2")
		case secretvault.KindHy2ObfsPasword:
			rep = testChannelSecret(task, snapshot, value, "obfs", meta.Fingerprint, "пароль obfs")
		case secretvault.KindVpsSSHKey:
			rep = testSSHKey(task, value, meta.Fingerprint)
		case secretvault.KindCfApiToken:
			rep = testCfToken(task, value, meta.Fingerprint)
		default:
			rep.Steps = []ProbeStepView{{Name: "сетевая проверка", Status: "skipped", Detail: "для этого вида сетевой сценарий не определён"}}
		}
	}
	value = "" // drops this reference only, not a memory-erasure claim
	if err := vault.WithPreparedSnapshot(snapshot, task.currentError); err != nil {
		return SecretTestReport{}, secretvault.Meta{}, errSecretTestUnavailable
	}
	return rep, meta, nil
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

// This legacy diagnostic still uses pinned SSH host keys, not the pending
// signed purpose-bound SSH trust delivery. No keyscan/TOFU/direct IP fallback.
// Capture one unambiguous descriptor host before dialing; never reread it during
// the handshake. A TCP open/SSH echo/hash prefix is not VPN-path acceptance.
func secretSSHAddress() (string, error) {
	channels, err := render.LoadDescriptors()
	if err != nil {
		return "", errSecretTestUnavailable
	}
	host := ""
	for _, ch := range channels {
		if !ch.Enabled || ch.Protocol != "vless" {
			continue
		}
		candidate := ch.OriginServer
		if candidate == "" || len(candidate) > 253 || strings.ContainsAny(candidate, " \t\r\n/\\?#@") || host != "" && candidate != host {
			return "", errSecretTestUnavailable
		}
		host = candidate
	}
	if host == "" {
		return "", errSecretTestUnavailable
	}
	return net.JoinHostPort(host, "22"), nil
}

const secretCommandLimit = 4096

// Session stdout and stderr are copied concurrently by x/crypto/ssh. Keep one
// synchronized, bounded sink; overflow cancels the original task/socket rather
// than retaining unbounded CombinedOutput or spawning a timeout waiter.
type secretCommandOutput struct {
	mu       sync.Mutex
	data     []byte
	overflow bool
	task     *manualDiagnosticTask
}

func (out *secretCommandOutput) Write(p []byte) (int, error) {
	out.mu.Lock()
	n := len(p)
	remaining := secretCommandLimit - len(out.data)
	if n > remaining {
		n = remaining
		out.overflow = true
	}
	out.data = append(out.data, p[:n]...)
	overflow := out.overflow
	out.mu.Unlock()
	if overflow {
		out.task.cancel()
		return n, io.ErrShortBuffer
	}
	return n, nil
}

func runSecretSSH(task *manualDiagnosticTask, addr string, signer ssh.Signer, command, stdin string) ([]byte, error) {
	conn, err := task.dialSSH(addr)
	if err != nil {
		return nil, errSecretTestUnavailable
	}
	defer conn.Close()
	cfg := &ssh.ClientConfig{User: testSSHUser, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: pinnedHostKeyCallback(), Timeout: testDialTimeout}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		return nil, errSecretTestUnavailable
	}
	client := ssh.NewClient(c, chans, reqs)
	defer func() {
		sequence := &appCleanupSequence{}
		defer func() {
			if sequence.result != nil {
				task.cleanupFailed.Store(true)
			}
		}()
		defer appCleanupStep("wait original SSH client", sequence, func() error { _ = client.Wait(); return nil })
		defer appCleanupStep("close original SSH client", sequence, func() error { _ = client.Close(); return nil })
	}()
	if task.currentError() != nil {
		return nil, errSecretTestUnavailable
	}
	sess, err := client.NewSession()
	if err != nil {
		return nil, errSecretTestUnavailable
	}
	defer sess.Close()
	out := &secretCommandOutput{task: task, data: make([]byte, 0, secretCommandLimit)}
	sess.Stdout, sess.Stderr = out, out
	if stdin != "" {
		sess.Stdin = strings.NewReader(stdin)
	}
	// Run waits for the command and its stdin/stdout/stderr copy functions. The
	// task remains admitted through this call and all connection/watcher defers.
	if task.currentError() != nil {
		return nil, errSecretTestUnavailable
	}
	err = sess.Run(command)
	out.mu.Lock()
	data, overflow := out.data, out.overflow
	out.mu.Unlock()
	if err != nil || overflow || task.currentError() != nil {
		return nil, errSecretTestUnavailable
	}
	return data, nil
}

func secretHashValid(value string) bool {
	if len(value) != 16 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func testChannelSecret(task *manualDiagnosticTask, snapshot *secretvault.PreparedImport, value, key, localFp, human string) SecretTestReport {
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	fail := func(step, detail string) SecretTestReport {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: step, Status: "fail", Detail: detail})
		return rep
	}
	kind := map[string]secretvault.Kind{"uuid": secretvault.KindVlessUUID, "hy2": secretvault.KindHy2Password, "obfs": secretvault.KindHy2ObfsPasword}[key]
	if err := secretvault.ValidateKind(kind, value); err != nil {
		return fail("формат значения", "значение не прошло проверку формата")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "формат значения", Status: "pass", Detail: human + ": формат корректен (fingerprint " + localFp + ")"})
	// Resolve the unique SSH credential from the SAME encrypted vault snapshot.
	// Ambiguous/missing credentials fail; never silently use ~/.ssh/vps_key.
	sshValue, err := snapshot.Get(string(secretvault.KindVpsSSHKey))
	if err != nil {
		return fail("SSH-ключ", "в этом снимке vault нет единственного доступного SSH-ключа")
	}
	signer, err := ssh.ParsePrivateKey([]byte(sshValue))
	sshValue = ""
	if err != nil {
		return fail("SSH-ключ", "SSH-ключ из снимка vault не разобран")
	}
	addr, err := secretSSHAddress()
	if err != nil {
		return fail("назначение SSH", "нет единственного адреса в доступных дескрипторах; запасной адрес не используется")
	}
	out, err := runSecretSSH(task, addr, signer, "python3", srvHashScript)
	if err != nil {
		return fail("SSH-запрос", "закреплённый SSH-сеанс и команда не подтверждены; причина по этому ответу не установлена")
	}
	var hashes srvHashes
	if json.Unmarshal(out, &hashes) != nil || !secretHashValid(hashes.UUID) || !secretHashValid(hashes.HY2) || !secretHashValid(hashes.Obfs) {
		return fail("серверный ответ", "ответ не соответствует ограниченному формату хешей")
	}
	got := map[string]string{"uuid": hashes.UUID, "hy2": hashes.HY2, "obfs": hashes.Obfs}[key]
	if got != localFp {
		return fail("сверка префикса", "сервер вернул другой SHA256-префикс; привязка конфигурации требует проверки")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "сверка префикса", Status: "pass", Detail: "SHA256-префикс ответа совпал со значением этого снимка. Это не проверка VPN-пути или всей конфигурации сервера."})
	rep.OK = true
	return rep
}

func testSSHKey(task *manualDiagnosticTask, value, localFp string) SecretTestReport {
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	fail := func(step, detail string) SecretTestReport {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: step, Status: "fail", Detail: detail})
		return rep
	}
	signer, err := ssh.ParsePrivateKey([]byte(value))
	if err != nil {
		return fail("разбор ключа", "ключ не разобран")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "разбор ключа", Status: "pass", Detail: "ключ разобран (fingerprint " + localFp + ")"})
	// Preserve the existing operator expectation refusal. This optional local
	// file is legacy metadata, NOT authenticated host-key rotation/delivery.
	exp, err := loadKeyExpectation(task)
	if err != nil {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: "локальное ожидание", Status: "unavailable", Detail: err.Error()}) // finite local errors only
		return rep
	}
	if exp != nil {
		if exp.PrivateKeyFingerprint != localFp {
			return fail("локальное ожидание", "fingerprint не совпадает с локальным ожиданием")
		}
		rep.Steps = append(rep.Steps, ProbeStepView{Name: "локальное ожидание", Status: "pass", Detail: "fingerprint локального значения совпал с ожиданием на момент чтения; это не аттестация сервера"})
	} else {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: "локальное ожидание", Status: "skipped", Detail: "оба допустимых файла ожиданий отсутствуют; локальная сверка не выполнялась"})
	}
	addr, err := secretSSHAddress()
	if err != nil {
		return fail("назначение SSH", "нет единственного адреса в доступных дескрипторах; запасной адрес не используется")
	}
	out, err := runSecretSSH(task, addr, signer, "echo ok", "")
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		return fail("SSH-запрос", "закреплённый SSH-сеанс и команда не подтверждены; причина по этому ответу не установлена")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "SSH-запрос", Status: "pass", Detail: "в этом сеансе сервер принял подпись и вернул ожидаемый ответ команды. VPN-путь не проверен."})
	rep.OK = true
	return rep
}

func testCfToken(task *manualDiagnosticTask, value, localFp string) SecretTestReport {
	rep := SecretTestReport{Steps: []ProbeStepView{}}
	fail := func(step, detail string) SecretTestReport {
		rep.Steps = append(rep.Steps, ProbeStepView{Name: step, Status: "fail", Detail: detail})
		return rep
	}
	if secretvault.ValidateKind(secretvault.KindCfApiToken, value) != nil {
		return fail("формат значения", "значение не прошло проверку формата")
	}
	if task.currentError() != nil {
		return fail("запрос", "проверка отменена")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "формат значения", Status: "pass", Detail: "формат корректен (fingerprint " + localFp + ")"})
	req, err := http.NewRequestWithContext(task.ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
	if err != nil {
		return fail("HTTP-запрос", "запрос не создан")
	}
	req.Header.Set("Authorization", "Bearer "+value)
	dialer := &net.Dialer{Timeout: testDialTimeout}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: task.ownedDialer(dialer.DialContext),
		TLSHandshakeTimeout: testDialTimeout, ResponseHeaderTimeout: testDialTimeout,
		MaxResponseHeaderBytes: 16 << 10, MaxConnsPerHost: 1, DisableCompression: true, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: testHTTPTimeout, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fail("HTTP-запрос", "ответ API не получен; причина по этому ответу не установлена")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail("ответ API", "API не подтвердил запрос (HTTP "+strconv.Itoa(resp.StatusCode)+"); статус токена не установлен")
	}
	const limit = 16 << 10
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(raw) > limit || task.currentError() != nil {
		return fail("ответ API", "ответ не прочитан в пределах разрешённого объёма и времени")
	}
	var body struct {
		Success bool `json:"success"`
		Result  *struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	// Unmarshal consumes one complete bounded JSON document, rejecting trailing data.
	if json.Unmarshal(raw, &body) != nil || !body.Success || body.Result == nil {
		return fail("ответ API", "API не передал подтверждённый статус токена")
	}
	if body.Result.Status != "active" {
		return fail("ответ API", "API не подтвердил активный статус; причина не установлена")
	}
	rep.Steps = append(rep.Steps, ProbeStepView{Name: "ответ API", Status: "pass", Detail: "API сообщил active для этого запроса. Права операций, будущая доступность и VPN-путь не проверены."})
	rep.OK = true
	return rep
}
