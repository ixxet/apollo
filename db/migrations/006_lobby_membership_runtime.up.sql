CREATE TABLE apollo.lobby_memberships (
    user_id UUID PRIMARY KEY REFERENCES apollo.users(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('joined', 'not_joined')),
    joined_at TIMESTAMPTZ,
    left_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (status = 'joined' AND joined_at IS NOT NULL)
        OR status = 'not_joined'
    )
);
