package onboarding

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

type SourceSelection struct {
	Repository  string `json:"repository"`
	TargetPath  string `json:"target_path"`
	Route       string `json:"route"` // protected, bootstrap, local
	AllowDirect bool   `json:"allow_direct"`
}

type DownloadedProfile struct {
	JSON       []byte // owned public bytes only; never a proof of client acceptance
	ReleaseID  string
	TargetPath string
}

type DownloadedPinPolicy struct {
	JSON      []byte // public capsule only; SDK/client acceptance still required
	ReleaseID string
}

type downloadedPublication struct {
	JSON       []byte
	ReleaseID  string
	TargetPath string
}

// The public head is an untrusted routing hint. This result conveys no new
// authority and never auto-applies a policy or changes the protected route.
var ErrPinPolicyPublication = errors.New("onboarding: текущая публикация содержит только отзывы сертификатов; примените список отзывов отдельно")

var errReleaseReplaced = errors.New("onboarding: selected public release was replaced")

func (s SourceSelection) Validate() (*url.URL, error) {
	return s.validateKind(delivery.Profile)
}

func (s SourceSelection) validateKind(kind delivery.Kind) (*url.URL, error) {
	if kind != delivery.Profile && kind != delivery.PinPolicy {
		return nil, delivery.ErrTrust
	}
	if len(s.Repository) == 0 || len(s.Repository) > 2048 || strings.ContainsAny(s.Repository, "%\\\r\n\t ") {
		return nil, errors.New("onboarding: укажите точный адрес репозитория без параметров входа")
	}
	u, err := url.Parse(s.Repository)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || u.Hostname() == "" {
		return nil, errors.New("onboarding: адрес репозитория не должен содержать пароль, query или fragment")
	}
	for _, part := range strings.Split(strings.Trim(u.Path, "/"), "/") {
		if part == "." || part == ".." || strings.HasSuffix(part, ".") {
			return nil, errors.New("onboarding: неоднозначный путь репозитория")
		}
		for _, ch := range part {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("._-", ch)) {
				return nil, errors.New("onboarding: поддерживается явный ASCII URL")
			}
		}
	}
	if err = (delivery.Request{Kind: kind, Path: s.TargetPath, Bytes: []byte("{}")}).Validate(); err != nil {
		return nil, err
	}
	ip, ipErr := netip.ParseAddr(u.Hostname())
	switch s.Route {
	case "local":
		if u.Scheme != "http" || ipErr != nil || !ip.IsLoopback() || ip.Zone() != "" || u.Port() == "" || s.AllowDirect {
			return nil, errors.New("onboarding: локальный источник — только явный HTTP loopback с портом")
		}
	case "protected", "bootstrap":
		if u.Scheme != "https" || s.Route == "protected" && s.AllowDirect || s.Route == "bootstrap" && !s.AllowDirect {
			return nil, errors.New("onboarding: HTTPS через выбранный маршрут; загрузка без VPN требует отдельного согласия")
		}
		if ipErr == nil {
			ip = ip.Unmap()
			if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.Zone() != "" {
				return nil, errors.New("onboarding: для локального источника выберите локальный маршрут")
			}
		} else {
			host := strings.ToLower(u.Hostname())
			if !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".") {
				return nil, errors.New("onboarding: требуется явное публичное HTTPS имя")
			}
			for _, label := range strings.Split(host, ".") {
				if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
					return nil, errors.New("onboarding: некорректное HTTPS имя")
				}
				for _, ch := range label {
					if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
						return nil, errors.New("onboarding: используйте ASCII/punycode HTTPS имя")
					}
				}
			}
		}
	default:
		return nil, errors.New("onboarding: выберите маршрут загрузки")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return nil, errors.New("onboarding: некорректный порт")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

// FetchPublicProfile does no trust enrollment, lifecycle or file/vault writes.
// Reselection happens only before SDK acceptance. A failed protected route is
// never retried directly; redirects, environment proxies and decompression are off.
func FetchPublicProfile(ctx context.Context, selection SourceSelection, ownedSOCKS string) (*DownloadedProfile, error) {
	result, err := fetchPublication(ctx, selection, ownedSOCKS, delivery.Profile)
	if err != nil {
		return nil, err
	}
	return &DownloadedProfile{JSON: result.JSON, ReleaseID: result.ReleaseID, TargetPath: result.TargetPath}, nil
}

func FetchPinPolicy(ctx context.Context, selection SourceSelection, ownedSOCKS string) (*DownloadedPinPolicy, error) {
	result, err := fetchPublication(ctx, selection, ownedSOCKS, delivery.PinPolicy)
	if err != nil {
		return nil, err
	}
	return &DownloadedPinPolicy{JSON: result.JSON, ReleaseID: result.ReleaseID}, nil
}

func fetchPublication(ctx context.Context, selection SourceSelection, ownedSOCKS string, kind delivery.Kind) (*downloadedPublication, error) {
	base, err := selection.validateKind(kind)
	if err != nil || ctx == nil {
		return nil, errors.Join(errors.New("onboarding: требуется корректный источник и контекст"), err)
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, DisableCompression: true, MaxResponseHeaderBytes: 16 << 10, ResponseHeaderTimeout: 8 * time.Second, TLSHandshakeTimeout: 8 * time.Second, MaxConnsPerHost: 1, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	if selection.Route == "protected" {
		host, port, err := net.SplitHostPort(ownedSOCKS)
		ip, ipErr := netip.ParseAddr(host)
		n, portErr := strconv.Atoi(port)
		if err != nil || ipErr != nil || !ip.IsLoopback() || ip.Zone() != "" || portErr != nil || n < 1 || n > 65535 {
			return nil, errors.New("onboarding: текущий защищённый SOCKS маршрут недоступен")
		}
		transport.Proxy = http.ProxyURL(&url.URL{Scheme: "socks5h", Host: ownedSOCKS})
	} else if ownedSOCKS != "" {
		return nil, errors.New("onboarding: маршрут источника изменился")
	}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for attempt := 0; attempt < 2; attempt++ {
		current := *base
		current.Path += "/current.json"
		raw, err := fetchPublicJSON(ctx, client, &current, 16<<10, "")
		if err != nil {
			return nil, err
		}
		head, err := delivery.DecodePublicationHead(raw)
		if err != nil {
			return nil, err
		}
		if kind == delivery.Profile && head.Purpose == delivery.PinPolicy {
			return nil, ErrPinPolicyPublication
		}
		if kind == delivery.PinPolicy && head.Purpose != delivery.PinPolicy {
			return nil, errors.New("onboarding: отдельный выпуск отзывов не опубликован")
		}
		sum := sha256.Sum256([]byte(string(kind) + "/" + selection.TargetPath))
		artifact := *base
		artifact.Path += "/" + head.Directory + "/clients/" + hex.EncodeToString(sum[:]) + ".json"
		limit := int64(MaxPublicProfileSize)
		if kind == delivery.PinPolicy {
			limit = MaxPinPolicyCapsuleSize
		}
		raw, err = fetchPublicJSON(ctx, client, &artifact, limit, head.ReleaseID)
		if errors.Is(err, errReleaseReplaced) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if kind == delivery.PinPolicy {
			if _, err = DecodePinPolicyCapsule(raw); err != nil {
				return nil, err
			}
		} else {
			payload, err := DecodePublicProfile(raw)
			if err != nil || payload == nil || payload.Delivery == nil || payload.Delivery.TargetPath != selection.TargetPath {
				return nil, errors.New("onboarding: публичный пакет не соответствует выбранному пути")
			}
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		return &downloadedPublication{JSON: raw, ReleaseID: head.ReleaseID, TargetPath: selection.TargetPath}, nil
	}
	return nil, errReleaseReplaced
}

func fetchPublicJSON(ctx context.Context, client *http.Client, address *url.URL, limit int64, release string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("onboarding: источник недоступен по выбранному маршруту")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone && release != "" {
		return nil, errReleaseReplaced
	}
	contentType, _, typeErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if resp.StatusCode != http.StatusOK || typeErr != nil || contentType != "application/json" || encoding != "" && encoding != "identity" || resp.ContentLength > limit {
		return nil, errors.New("onboarding: источник не вернул допустимый ограниченный JSON")
	}
	if got := resp.Header.Get("X-Snowden-Release"); release != "" && got != "" && got != release {
		return nil, errors.New("onboarding: выпуск изменился во время загрузки")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(raw) == 0 || int64(len(raw)) > limit {
		return nil, errors.New("onboarding: публичный пакет не получен полностью или превышает лимит")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return raw, nil
}
