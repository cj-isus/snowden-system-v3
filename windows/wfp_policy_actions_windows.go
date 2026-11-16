//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/tailscale/wf"
	"golang.org/x/sys/windows"
)

// Initial base publication is one native transaction. Existing protection is
// inspected/preserved; retries never tear it down or repair damaged policy into
// a successful Arm. Partial pre-transaction legacy objects need Disconnect.
func applyWFPBase(exePath string) error {
	ctx := context.Background()
	appID, err := readOwnedWFPAppID(ctx, exePath)
	if err != nil {
		return err
	}
	return withWFPPolicyWriter(ctx, wfpPolicyArm, func(owner *nativeWFPPolicyOwner) error {
		objects, err := owner.session.inspectObjects(ctx)
		if err != nil {
			return err
		}
		rules, _, err := owner.session.readPolicy(ctx, false)
		if err != nil {
			return err
		}
		if len(rules) != 0 {
			if err := verifyWFPScopeContext(ctx, false, 0, nil, false, &owner.session); err != nil {
				return err
			}
			return deleteGeneralWFPPermits(owner, rules)
		}
		if objects.providerPresent || objects.sublayerPresent {
			return fmt.Errorf("WFP partial existing policy requires explicit Disconnect")
		}
		if err := owner.addProvider(); err != nil {
			return err
		}
		if err := owner.addSublayer(); err != nil {
			return err
		}
		for _, spec := range wfpBaseRuleSpecs(appID) {
			if err := owner.addRule(spec); err != nil {
				return err
			}
		}
		return nil
	})
}

func permitWFPTUN(name string, timeout time.Duration) error {
	_, err := permitWFPTUNContext(context.Background(), name, timeout)
	return err
}

func permitWFPTUNContext(ctx context.Context, name string, timeout time.Duration) (uint64, error) {
	luid, err := waitInterfaceLUIDContext(ctx, name, timeout)
	if err != nil {
		return 0, err
	}
	err = grantWFPTUNLUIDContext(ctx, luid, false)
	if err != nil {
		return 0, err
	}
	return luid, nil
}

// Native lifecycle uses its previously observed exact LUID and a strict closed
// read inside the SAME write transaction. It cannot adopt matching old permits.
func permitPreparedWFPTUNContext(ctx context.Context, luid uint64) error {
	return grantWFPTUNLUIDContext(ctx, luid, true)
}

func grantWFPTUNLUIDContext(ctx context.Context, luid uint64, requireClosed bool) error {
	if ctx == nil || ctx.Err() != nil || luid == 0 {
		return core.ErrNativeScopeUnavailable
	}
	return withWFPPolicyWriter(ctx, wfpPolicyGrant, func(owner *nativeWFPPolicyOwner) error {
		owner.batch.expectedLUID = luid
		if err := verifyWFPScopeContext(ctx, false, 0, nil, requireClosed, &owner.session); err != nil {
			return err
		}
		rules, _, err := owner.session.readPolicy(ctx, false)
		if err != nil {
			return err
		}
		present := 0
		for _, spec := range wfpTunRuleSpecs(luid) {
			for _, rule := range rules {
				if rule.ID == wf.RuleID(wfpStableGUID(spec.id)) {
					if !wfpRuleMatches(rule, spec) {
						return fmt.Errorf("WFP prior general permit requires restrictive cleanup")
					}
					present++
				}
			}
		}
		if present == 2 {
			return nil // fully matching existing set: no delete/regrant interruption
		}
		if present != 0 {
			return fmt.Errorf("WFP partial general permits require restrictive cleanup")
		}
		for _, spec := range wfpTunRuleSpecs(luid) {
			// Missing owned IDs are added only; a foreign collision fails Add.
			if err := owner.addRule(spec); err != nil {
				return err
			}
		}
		return nil
	})
}

// BlockDirect removes only general permits. It is the allowed restrictive
// recovery after a failed/unknown grant commit, on the SAME original session.
// The diagnostic actor's dynamic session is retired by its own protected lane.
func revokeWFPTUN() error {
	return revokeWFPTUNContext(context.Background())
}

func revokeWFPTUNContext(ctx context.Context) error {
	return withWFPPolicyWriter(ctx, wfpPolicyRestrict, func(owner *nativeWFPPolicyOwner) error {
		rules, err := owner.session.readRuleIdentities(ctx)
		if err != nil {
			return err
		}
		return deleteGeneralWFPPermits(owner, rules)
	})
}

func deleteGeneralWFPPermits(owner *nativeWFPPolicyOwner, rules []*wf.Rule) error {
	for _, rule := range rules {
		if rule.ID != wf.RuleID(wfpStableGUID(20)) && rule.ID != wf.RuleID(wfpStableGUID(21)) {
			continue
		}
		if rule.Provider != wfpProviderID || !rule.Persistent {
			return core.ErrNativeScopeUnavailable
		}
		if err := owner.deleteObject(wfpDeleteFilter, windows.GUID(rule.ID)); err != nil {
			return err
		}
	}
	return nil
}

// Called only by explicit Disconnect after engine/helper retirement. Identity
// rows allow removal of damaged OWNED rules, never validation or foreign delete.
func removeWFPPolicy() error {
	ctx := context.Background()
	return withWFPPolicyWriter(ctx, wfpPolicyRelease, func(owner *nativeWFPPolicyOwner) error {
		objects, err := owner.session.inspectObjects(ctx)
		if err != nil {
			return err
		}
		if objects.sublayerPresent && !objects.sublayerOwned {
			return fmt.Errorf("WFP sublayer belongs to another provider")
		}
		rules, err := owner.session.readRuleIdentities(ctx)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if rule.Provider != wfpProviderID || !rule.Persistent {
				return fmt.Errorf("WFP active dynamic owner must retire before Disconnect")
			}
			if err := owner.deleteObject(wfpDeleteFilter, windows.GUID(rule.ID)); err != nil {
				return err
			}
		}
		if objects.sublayerPresent {
			if err := owner.deleteObject(wfpDeleteSublayer, windows.GUID(wfpSublayerID)); err != nil {
				return err
			}
		}
		if objects.providerPresent {
			return owner.deleteObject(wfpDeleteProvider, owner.session.provider)
		}
		return nil
	})
}

func verifyWFPPolicyAction(ctx context.Context, owner *nativeWFPPolicyOwner, action wfpPolicyAction) error {
	switch action {
	case wfpPolicyArm:
		return verifyWFPScopeContext(ctx, false, 0, nil, true, &owner.session)
	case wfpPolicyGrant:
		if owner.batch.expectedLUID == 0 {
			return core.ErrNativeScopeUnavailable
		}
		return verifyWFPScopeContext(ctx, true, owner.batch.expectedLUID, nil, false, &owner.session)
	case wfpPolicyRestrict:
		rules, err := owner.session.readRuleIdentities(ctx)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if rule.ID == wf.RuleID(wfpStableGUID(20)) || rule.ID == wf.RuleID(wfpStableGUID(21)) {
				return fmt.Errorf("WFP general permit retirement unconfirmed")
			}
		}
		return ctx.Err()
	case wfpPolicyRelease:
		return verifyWFPPolicyAbsent(ctx, &owner.session)
	}
	return core.ErrNativeScopeUnavailable
}

func verifyWFPPolicyAbsent(ctx context.Context, session *nativeWFPSessionOwner) (finalErr error) {
	ownsTransaction := session.transaction == nil || session.transaction.phase == wfpTransactionFinished
	if ownsTransaction {
		defer func() { finalErr = errors.Join(finalErr, session.abortTransaction(), ctx.Err()) }()
		if err := session.beginTransaction(true); err != nil {
			return err
		}
	}
	objects, err := session.inspectObjects(ctx)
	if err != nil {
		return err
	}
	rules, err := session.readRuleIdentities(ctx)
	if err != nil {
		return err
	}
	if objects.providerPresent || objects.sublayerPresent || len(rules) != 0 {
		return fmt.Errorf("WFP owned policy removal unconfirmed")
	}
	return ctx.Err()
}
