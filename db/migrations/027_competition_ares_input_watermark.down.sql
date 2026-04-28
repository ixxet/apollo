DROP INDEX IF EXISTS apollo.idx_competition_match_previews_session_input_watermark;

ALTER TABLE apollo.competition_match_previews
    RENAME COLUMN input_watermark TO generated_at;

CREATE INDEX idx_competition_match_previews_session_generated_at
    ON apollo.competition_match_previews (competition_session_id, generated_at DESC, id DESC);
