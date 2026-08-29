-- +goose Up

-- The console could not read the scheduled reports at all.
--
-- Its front page counts them — how many have never run, how many failed —
-- because a scheduled report that stops arriving is noticed weeks later by
-- the person who was expecting it. That count has been coming back as a
-- swallowed `permission denied`: observability.backgroundJobs logs a warning
-- and shows a healthy-looking panel, which is the exact failure the panel was
-- built to catch.
--
-- Read-only and across every organisation: the console shows which schedule is
-- broken, it does not edit anybody's.
GRANT SELECT ON workspace.report_schedules TO gerege_nexus_operator;

DROP POLICY IF EXISTS operator_read ON workspace.report_schedules;
CREATE POLICY operator_read ON workspace.report_schedules FOR SELECT TO gerege_nexus_operator
    USING (true);

-- +goose Down

DROP POLICY IF EXISTS operator_read ON workspace.report_schedules;
REVOKE ALL ON workspace.report_schedules FROM gerege_nexus_operator;
