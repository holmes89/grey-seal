-- +goose Up

-- After a draft is kept, chat offers to move onto the next work type
-- (discovery -> design -> tickets). The offer must survive between requests
-- so the user's "yes" can be recognized; source is the kept draft's content,
-- used to seed the next draft.
ALTER TABLE conversations ADD COLUMN pending_chain_offer TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN pending_chain_offer_source TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE conversations DROP COLUMN pending_chain_offer_source;
ALTER TABLE conversations DROP COLUMN pending_chain_offer;
