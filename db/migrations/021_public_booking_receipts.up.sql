CREATE TABLE apollo.public_booking_receipts (
    receipt_code TEXT PRIMARY KEY,
    booking_request_id UUID NOT NULL UNIQUE REFERENCES apollo.booking_requests(id) ON DELETE RESTRICT,
    public_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT public_booking_receipts_code_required
        CHECK (btrim(receipt_code) <> ''),
    CONSTRAINT public_booking_receipts_message_safe_shape
        CHECK (
            public_message IS NULL
            OR (
                btrim(public_message) <> ''
                AND length(public_message) <= 1000
            )
        )
);

INSERT INTO apollo.public_booking_receipts (
    receipt_code,
    booking_request_id,
    created_at,
    updated_at
)
SELECT 'BR-' || upper(replace(gen_random_uuid()::text, '-', '')),
       id,
       created_at,
       updated_at
FROM apollo.booking_requests
WHERE request_source = 'public'
ON CONFLICT (booking_request_id) DO NOTHING;
