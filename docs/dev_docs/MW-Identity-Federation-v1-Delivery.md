# Federation v1 delivery record

The frozen contract remains normative. Each PR increment is a separate local commit. Deployment and consumer evidence remain separate from source implementation. Never advertise full compliance while any required live gate is unverified.

## PR-00

Recorded baseline, preserved frozen contract, copied supplied implementation plan and added the dependency inventory and A01–A28 baseline map. See the compliance document for exact test outcomes and unresolved live checks.

## PR-01

Migration 014 adds `access_policy_v1` without changing existing registrations. New applications created by the service use v1 rules: Public defaults allow, Private requires assignment, explicit deny wins, ordinary-app superadmins receive no exemption. SQL eligibility is shared across credential insert/read/consume paths. Existing legacy clients remain explicitly noncompliant until reviewed cutover, avoiding guesses about their intended signup-based access. Global registration controls app-initiated registration too; private-login signup invitations are hidden and restriction wording is bilingual. Direct global registration remains available.

Validation: Identity `go test ./...`, focused portal/store tests and `go vet ./...` pass. The new store integration test compares Go authorization and SQL issuance/read results for unassigned and explicitly denied superadmins and rejects signup bypass after cutover. No production database used.

Rollout: back up; perform the client audit in the compliance document; establish explicit Private assignments first; run `/app/federation-cutover --client-id <reviewed-client> --access public|private --reviewed`. The command atomically changes the reviewed policy and revokes existing client codes/tokens, with an audit record. Existing client ID/secret stays intact. Observe fresh authorization before the next cutover. Live audit is unverified here.

Rollback: revert code while retaining additive columns; restore the recorded access setting only through an operator-reviewed change. Old binaries do not honor `access_policy_v1`, so reverting may restore legacy signup/superadmin access bypasses. Do not use rollback as an implicit policy change. No schema downgrade or historical lifecycle replay.
