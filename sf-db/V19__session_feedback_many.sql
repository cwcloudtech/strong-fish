-- A session may be read more than once.
--
-- It was one reading per session, replaced on demand: the answer costs money,
-- two askings give two different readings, and keeping one was how a member
-- came back to what they were told. That holds - but it also made the readings
-- mutually exclusive, and they are not. A lifter wants a second opinion from a
-- bigger model; a coach wants to add their own reading beside their athlete's
-- without deleting it. So the pair stops being unique and a session carries as
-- many readings as were asked for, each with its own author and model.
--
-- Its own migration rather than an edit to V17: that one is committed, and a
-- migration that has been applied anywhere is frozen - Flyway checksums it,
-- and a deployment that has already run it would refuse to start.
ALTER TABLE session_feedback DROP CONSTRAINT session_feedback_assignment_id_day_id_key;

-- Reading a session's feedback is reading all of it, newest first, which is
-- what the constraint's index used to serve for free.
CREATE INDEX idx_session_feedback_day ON session_feedback(assignment_id, day_id, created_at DESC);
