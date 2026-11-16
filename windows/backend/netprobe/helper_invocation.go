package netprobe

import "encoding/base64"

type HelperRole uint8

const (
	HelperDNS HelperRole = iota + 1
	HelperTUN
)

// HelperInvocation is a closed bounded argv header. It does not authenticate the
// parent/Job/image or decode native packets. TUN subject/nonce stay on stdin.
type HelperInvocation struct {
	role  HelperRole
	token string
}

func ParseHelperInvocation(args []string) (HelperInvocation, error) {
	if len(args) == 1 && args[0] == TUNArgument {
		return HelperInvocation{role: HelperTUN}, nil
	}
	if len(args) == 2 && args[0] == NativeArgument && len(args[1]) >= 1 && len(args[1]) <= base64.RawURLEncoding.EncodedLen(MaxNativeRequest) {
		return HelperInvocation{role: HelperDNS, token: args[1]}, nil
	}
	return HelperInvocation{}, ErrNativeProtocol
}

func NewDNSInvocation(request NativeRequest) (HelperInvocation, error) {
	token, err := EncodeNativeRequest(request)
	if err != nil {
		return HelperInvocation{}, err
	}
	return ParseHelperInvocation([]string{NativeArgument, token})
}

func NewTUNInvocation() HelperInvocation             { return HelperInvocation{role: HelperTUN} }
func (invocation HelperInvocation) Role() HelperRole { return invocation.role }

func (invocation HelperInvocation) Arguments() ([]string, error) {
	switch invocation.role {
	case HelperDNS:
		if len(invocation.token) < 1 || len(invocation.token) > base64.RawURLEncoding.EncodedLen(MaxNativeRequest) {
			return nil, ErrNativeProtocol
		}
		return []string{NativeArgument, invocation.token}, nil
	case HelperTUN:
		return []string{TUNArgument}, nil
	default:
		return nil, ErrNativeProtocol
	}
}

// Decode only after the process entry's actual pipe/Job precondition. The exact
// existing strict bounded decoder remains the sole DNS request parser.
func (invocation HelperInvocation) DNSRequest() (NativeRequest, error) {
	if invocation.role != HelperDNS {
		return NativeRequest{}, ErrNativeProtocol
	}
	return DecodeNativeRequest(invocation.token)
}
