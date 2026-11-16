package main

import (
	"context"
	"os"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/netprobe"
)

func main() { os.Exit(run()) }

func run() (code int) {
	finished := false
	code = 2
	defer func() {
		if recovered := recover(); recovered != nil || !finished {
			code = 2
		}
	}()
	if childbootstrap.BrokerInvocationRequested(os.Args[1:]) {
		code = runBrokerInvocation(os.Args[1:])
		finished = true
		return code // reserved broker failures never fall through to DNS/TUN
	}
	invocation, err := netprobe.ParseHelperInvocation(os.Args[1:])
	if err != nil || !pipeJobReady() {
		return 2
	}
	if invocation.Role() == netprobe.HelperTUN {
		code := runNativeTUNInvocation()
		finished = true
		return code
	}
	request, err := invocation.DNSRequest()
	if err != nil {
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(request.TimeoutMillis)*time.Millisecond)
	defer cancel()
	reply := executeNative(ctx, request)
	raw, err := netprobe.EncodeNativeReply(reply, request)
	if err != nil {
		return 2
	}
	n, err := os.Stdout.Write(raw)
	if err != nil || n != len(raw) {
		return 2
	}
	finished = true
	return 0
}
