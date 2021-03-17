-- Style changes
-- I wanted to depluralize "Notes" and rolled with it.
-- Partially exists just to test the migrations

pragma foreign_keys = on;
alter table Notes rename to "note";
alter table "note" rename column Name to name;
alter table "note" rename column Body to body;
alter table "note" rename column CreateTime to create_time;
alter table "note" rename column LastViewed to last_viewed;
