-- A private message became a reportable target (models.ReportTargetMessage)
-- after V1, but V1's content_reports.target_type CHECK only ever allowed
-- 'post', 'comment' and 'user' - so reporting a message failed the constraint
-- (content_reports_target_type_check, SQLSTATE 23514). Widen it to include
-- 'message'.
ALTER TABLE content_reports DROP CONSTRAINT content_reports_target_type_check;
ALTER TABLE content_reports ADD CONSTRAINT content_reports_target_type_check
    CHECK (target_type IN ('post', 'comment', 'user', 'message'));
