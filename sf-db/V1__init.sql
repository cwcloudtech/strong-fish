-- strong-fish initial schema.
--
-- The shape follows cwclock's: every table is a thin set of indexed,
-- foreign-keyed columns plus a single `data` JSONB payload holding
-- everything the database itself never needs to join or filter on. That
-- keeps the schema stable while the domain (a set's prescription, a post's
-- attachments, an exercise's translated labels) keeps growing.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- Accounts
-- ---------------------------------------------------------------------------

-- data: {password, name, surname, role, handle, bio, picture, pictureX,
--        pictureY, locale, publicProfile, bodyweight, mfaEnabled,
--        mfaTotpSecret}
-- role is the account-wide role: superadmin | coach | confirmed | disabled |
-- ban. "confirmed" is a plain athlete/member; "coach" additionally owns clubs
-- and authors programs and exercises.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The public profile is addressed by handle rather than by id, so the URL a
-- member shares stays readable. Partial-unique: accounts that never picked
-- one (NULL) don't collide with each other.
CREATE UNIQUE INDEX idx_users_handle ON users((data->>'handle')) WHERE data->>'handle' IS NOT NULL;

CREATE TABLE webauthn_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    sign_count BIGINT NOT NULL DEFAULT 0,
    transports TEXT[] NOT NULL DEFAULT '{}',
    name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webauthn_credentials_user_id ON webauthn_credentials(user_id);

-- ---------------------------------------------------------------------------
-- Clubs
-- ---------------------------------------------------------------------------

-- A club is cwclock's organization: the coach who creates it is its owner,
-- members can be promoted to admin, and everything a coach uploads (programs)
-- lives inside one.
-- data: {name, description, city, country, picture, pictureX, pictureY}
CREATE TABLE clubs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_clubs_owner_id ON clubs(owner_id);

CREATE TABLE club_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (club_id, user_id)
);

CREATE INDEX idx_club_members_user_id ON club_members(user_id);

-- ---------------------------------------------------------------------------
-- Exercises
-- ---------------------------------------------------------------------------

-- The exercise catalog is global, not per-club: a coach adds "larsen press"
-- once and every other coach gets it in their autocomplete when filling a
-- program.
--
-- data: {slug, aliases, labels: {en, fr}, category, oneRmRef, bodyweight,
--        createdBy}
--   slug     - normalized name, the autocomplete/import lookup key
--   aliases  - extra normalized names resolving to this exercise, so a
--              spreadsheet spelling the movement differently (or with a typo)
--              imports onto the existing entry instead of forking a duplicate
--   labels   - display name per locale (en is the fallback)
--   category - squat | bench | deadlift | accessory
--   oneRmRef - which main lift's 1RM a percentage/RPE prescription for this
--              movement is computed against (squat/bench/deadlift), or null
--              for accessories loaded in absolute kilos
--   bodyweight - true for movements loaded by the athlete's own body (pull-ups,
--              dips): they carry reps and RPE but no barbell load
CREATE TABLE exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_exercises_slug ON exercises((data->>'slug'));
CREATE INDEX idx_exercises_category ON exercises((data->>'category'));

-- Each member keeps their own 1RM per exercise and may revise it as often as
-- they like; every prescribed set is (re)computed against the current value,
-- so an update propagates to the whole program with no stored derived load to
-- migrate. data: {value, unit, history: [{value, at}]}
CREATE TABLE one_rms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, exercise_id)
);

CREATE INDEX idx_one_rms_user_id ON one_rms(user_id);

-- ---------------------------------------------------------------------------
-- Programs
-- ---------------------------------------------------------------------------

-- data: {name, description, weeks, sourceFileName}
CREATE TABLE programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id UUID NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_programs_club_id ON programs(club_id);

-- One training session: "WEEK 2 DAY 3" in the source spreadsheet.
-- data: {week, day, title, position}
CREATE TABLE program_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_program_days_program_id ON program_days(program_id);

-- One prescribed set. The load is never stored: it's derived per member from
-- their own current 1RM (see loadcalc in sf-api), which is exactly what makes
-- "update your 1RM and the whole program recomputes" free.
--
-- data: {position, reps, rpe, percentage, absoluteLoad, loadMode, part, notes}
--   loadMode - rpe        : reps @ rpe against the member's 1RM (authoritative
--                           when the coach prescribed an RPE)
--              percentage : a fixed % of the member's 1RM, for sets the coach
--                           left without an RPE
--              absolute   : a fixed weight in kg, for accessories
--              bodyweight : no external load
--   percentage is kept alongside an rpe prescription as the coach's authored
--   value, so the UI can show what the spreadsheet said next to what the
--   member's 1RM actually implies.
--   part - optional grouping index used by the source spreadsheet to split a
--          session into blocks (main work / accessories).
CREATE TABLE program_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    day_id UUID NOT NULL REFERENCES program_days(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE RESTRICT,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_program_sets_program_id ON program_sets(program_id);
CREATE INDEX idx_program_sets_day_id ON program_sets(day_id);
CREATE INDEX idx_program_sets_exercise_id ON program_sets(exercise_id);

-- A program handed to one member. data: {startDate, status, note}
CREATE TABLE program_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (program_id, user_id)
);

CREATE INDEX idx_program_assignments_user_id ON program_assignments(user_id);

-- What the member actually did on a prescribed set, plus the RPE they
-- perceived and a comment for their coach.
-- data: {actualReps, actualRpe, actualLoad, comment, done, completedAt}
CREATE TABLE set_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id UUID NOT NULL REFERENCES program_assignments(id) ON DELETE CASCADE,
    set_id UUID NOT NULL REFERENCES program_sets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (assignment_id, set_id)
);

CREATE INDEX idx_set_logs_user_id ON set_logs(user_id);
CREATE INDEX idx_set_logs_set_id ON set_logs(set_id);

-- ---------------------------------------------------------------------------
-- Social
-- ---------------------------------------------------------------------------

-- club_id NULL means the post is public (visible to everyone, including
-- logged-out visitors on a public profile); a club id restricts it to that
-- club's members.
-- data: {content, pictures: [b64 data URI], links: [url], visibility}
CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    club_id UUID REFERENCES clubs(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_posts_author_id ON posts(author_id);
CREATE INDEX idx_posts_club_id ON posts(club_id);
CREATE INDEX idx_posts_created_at ON posts(created_at DESC);

CREATE TABLE post_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_post_comments_post_id ON post_comments(post_id);
CREATE INDEX idx_post_comments_author_id ON post_comments(author_id);

CREATE TABLE post_likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (post_id, user_id)
);

CREATE INDEX idx_post_likes_post_id ON post_likes(post_id);

CREATE TABLE follows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (follower_id, following_id),
    CHECK (follower_id <> following_id)
);

CREATE INDEX idx_follows_following_id ON follows(following_id);

-- target_type/target_id are a soft reference (a report outlives the content it
-- denounces, so a superadmin can still see what was moderated).
-- data: {reason, comment, status, resolvedBy, resolvedAt, snapshot}
CREATE TABLE content_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('post', 'comment', 'user')),
    target_id UUID NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_content_reports_target ON content_reports(target_type, target_id);
CREATE INDEX idx_content_reports_status ON content_reports((data->>'status'));
