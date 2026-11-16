package repository

import (
	"errors"
	"time"
)

// Run is shared by the existing metadata/update CLIs. No subcommand is run at
// package initialization. Only explicit sign/keygen read or create secret keys;
// Explicit publisher operations write the separate store; supplementation may
// retain a verified prefix, and serving may checkpoint its local clock floor.
func Run(args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, errors.New("repository: choose keygen, draft, sign, assemble, release, supplement-roots, promote, authorize-policy, recover, status, repair-aliases, maintenance-plan, maintenance-apply, backup, restore or serve")
	}
	now := time.Now().UTC()
	switch args[0] {
	case "restore":
		if len(args) != 9 {
			return nil, errors.New("repository restore <backup.zip> <approved-backup-sha256> <application-policy.json> <approved-policy-sha256> <approved-archived-state-sha256> <approved-anchor-sha256> <new|resume|approved-existing-target-state-sha256> <publisher-directory>")
		}
		return restorePublisher(args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8])
	case "maintenance-plan", "backup":
		if len(args) != 4 {
			return nil, errors.New("repository maintenance-plan|backup <approved-retained-state-sha256> <publisher-directory> <new-output>")
		}
		if args[0] == "backup" {
			return backupPublisher(args[1], args[2], args[3])
		}
		return planPublisherMaintenance(args[1], args[2], args[3])
	case "maintenance-apply":
		if len(args) != 4 {
			return nil, errors.New("repository maintenance-apply <plan.json> <approved-plan-sha256> <publisher-directory>")
		}
		return applyPublisherMaintenance(args[1], args[2], args[3])
	case "supplement-roots":
		if len(args) != 9 {
			return nil, errors.New("repository supplement-roots <candidate.json> <approved-candidate-sha256> <application-policy.json> <approved-policy-sha256> <approved-retained-state-sha256> <first-client-root-version> <publisher-directory> <new-candidate.json>")
		}
		return supplementRelease(args[1], args[2], args[3], args[4], args[5], args[6], args[7], args[8])
	case "status":
		if len(args) != 2 {
			return nil, errors.New("repository status <publisher-directory>")
		}
		return statusPublisher(args[1])
	case "repair-aliases":
		if len(args) != 3 {
			return nil, errors.New("repository repair-aliases <approved-retained-state-sha256> <publisher-directory>")
		}
		return repairPublisherAliases(args[1], args[2])
	case "serve":
		if len(args) != 3 {
			return nil, errors.New("repository serve <publisher-directory> <literal-loopback-address:port>")
		}
		return servePublisher(args[1], args[2])
	case "authorize-policy":
		if len(args) != 5 {
			return nil, errors.New("repository authorize-policy <application-policy.json> <approved-new-policy-sha256> <approved-current-policy-sha256> <publisher-directory>")
		}
		return authorizePublisherPolicy(args[1], args[2], args[3], args[4])
	case "promote":
		if len(args) != 7 {
			return nil, errors.New("repository promote <candidate.json> <approved-candidate-sha256> <application-policy.json> <approved-policy-sha256> <approved-first-root-sha256|retained> <publisher-directory>")
		}
		return promote(args[1], args[2], args[3], args[4], args[5], args[6])
	case "recover":
		if len(args) != 2 {
			return nil, errors.New("repository recover <publisher-directory>")
		}
		return recoverPublisher(args[1])
	case "release":
		if len(args) != 4 {
			return nil, errors.New("repository release <public-release-plan.json> <approved-issuer-root-sha256> <new-candidate.json>")
		}
		return release(args[1], args[2], args[3])
	case "keygen":
		if len(args) != 3 {
			return nil, errors.New("repository keygen <new-public.json> <new-private.key>")
		}
		return keygen(args[1], args[2])
	case "draft":
		if len(args) != 5 {
			return nil, errors.New("repository draft <public-plan.json> <approved-parent-sha|bootstrap> <approved-previous-sha|first> <new-draft.json>")
		}
		return draft(args[1], args[2], args[3], args[4], now)
	case "sign":
		if len(args) != 6 {
			return nil, errors.New("repository sign <role> <draft.json> <approved-draft-sha> <@private.key> <new-partial.json>")
		}
		return signDraft(args[1], args[2], args[3], args[4], args[5], now)
	case "assemble":
		if len(args) != 5 {
			return nil, errors.New("repository assemble <public-plan.json> <approved-draft-sha> <approved-parent-sha|bootstrap> <new-signed.json>")
		}
		return assemble(args[1], args[2], args[3], args[4], now)
	default:
		return nil, errors.New("repository: unknown operation")
	}
}
