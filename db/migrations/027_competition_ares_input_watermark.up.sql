DROP INDEX IF EXISTS apollo.idx_competition_match_previews_session_generated_at;

ALTER TABLE apollo.competition_match_previews
    RENAME COLUMN generated_at TO input_watermark;

CREATE INDEX idx_competition_match_previews_session_input_watermark
    ON apollo.competition_match_previews (competition_session_id, input_watermark DESC, id DESC);
