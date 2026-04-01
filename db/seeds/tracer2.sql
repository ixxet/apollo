INSERT INTO apollo.users (
    id,
    student_id,
    display_name,
    email
)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'tracer2-student-001',
    'Tracer Two',
    'tracer2-student-001@example.com'
)
ON CONFLICT (student_id) DO NOTHING;

INSERT INTO apollo.claimed_tags (
    id,
    user_id,
    tag_hash,
    label,
    is_active
)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    '11111111-1111-1111-1111-111111111111',
    'tag_tracer2_001',
    'Tracer 2 Mock Tag',
    TRUE
)
ON CONFLICT (tag_hash) DO NOTHING;
