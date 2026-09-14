package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPreflightResolveIPLiteral(t *testing.T) {
	r := &PreflightResolver{}
	ip, cached, err := r.Resolve(context.Background(), "203.0.113.10")
	if err != nil || cached || ip != "203.0.113.10" {
		t.Fatalf("ip-literal: ip=%q cached=%v err=%v", ip, cached, err)
	}
}

func TestPreflightDoHParsing(t *testing.T) {
	ip, err := parseDoHAnswer("1.1.1.1", []byte(`{"Status":0,"Answer":[{"type":1,"data":"93.184.216.34"}]}`))
	if err != nil || ip != "93.184.216.34" {
		t.Fatalf("parse: %q %v", ip, err)
	}
	// DNS-level отказ (NXDOMAIN и т.п.) и пустой ответ — ошибки.
	if _, err := parseDoHAnswer("1.1.1.1", []byte(`{"Status":3,"Answer":[]}`)); err == nil {
		t.Fatal("status!=0 must fail")
	}
	if _, err := parseDoHAnswer("1.1.1.1", []byte(`{"Status":0,"Answer":[]}`)); err == nil {
		t.Fatal("empty answer must fail")
	}
	// AAAA-only/bogon ответы — тоже (нужен публичный IPv4).
	if _, err := parseDoHAnswer("1.1.1.1", []byte(`{"Status":0,"Answer":[{"type":28,"data":"2606:4700::1"}]}`)); err == nil {
		t.Fatal("ipv6-only answer must fail for ipv4 strategy")
	}
	if _, err := parseDoHAnswer("1.1.1.1", []byte(`{"Status":0,"Answer":[{"type":1,"data":"192.168.0.1"}]}`)); err == nil {
		t.Fatal("bogon answer must fail")
	}
}

func TestPreflightCacheRoundTripAndTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	r := &PreflightResolver{CachePath: path}
	r.cachePut("a.example", "1.2.3.4")
	if ip, ok := r.cacheGet("a.example"); !ok || ip != "1.2.3.4" {
		t.Fatalf("cache get: %q %v", ip, ok)
	}
	// Файл существует и не пуст.
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	// Просроченный кэш не отдаётся.
	r.cachePut("b.example", "5.6.7.8")
	cache := r.loadCache()
	e := cache["b.example"]
	e.At = time.Now().Add(-preflightCacheTTL - time.Hour)
	cache["b.example"] = e
	data, _ := jsonMarshal(cache)
	_ = os.WriteFile(path, data, 0o600)
	if _, ok := r.cacheGet("b.example"); ok {
		t.Fatal("expired entry must not be returned")
	}
}

func TestPreflightFirstPublicIPv4SkipsBogons(t *testing.T) {
	if ip, err := firstPublicIPv4([]string{"10.0.0.1", "192.168.1.1", "127.0.0.1", "93.184.216.34"}); err != nil || ip != "93.184.216.34" {
		t.Fatalf("got %q %v", ip, err)
	}
	if _, err := firstPublicIPv4([]string{"10.0.0.1"}); err == nil {
		t.Fatal("bogon-only answer must fail")
	}
}
