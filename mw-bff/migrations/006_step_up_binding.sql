ALTER TABLE bff_oidc_transactions ADD COLUMN expected_subject TEXT NOT NULL DEFAULT '';
ALTER TABLE bff_oidc_transactions ADD COLUMN required_acr TEXT NOT NULL DEFAULT '';
