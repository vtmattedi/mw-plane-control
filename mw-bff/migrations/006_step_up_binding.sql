ALTER TABLE bff_oidc_transactions ADD COLUMN expected_subject TEXT NOT NULL DEFAULT '';
ALTER TABLE bff_oidc_transactions ADD COLUMN required_acr TEXT NOT NULL DEFAULT '';
-- Legacy transactions do not identify whether they requested step-up or its subject.
-- Restart sign-in rather than accepting unbound in-flight assurance after upgrade.
DELETE FROM bff_oidc_transactions WHERE consumed_at_unix IS NULL;
