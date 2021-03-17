-- NOTE: This document is for reference only & is not canonical.
-- The actual schema is the concatenation of all the migrations, in order.

create table _migration (
	date    date,
	number  number,
	primary key (date, number)
);

create table "note" (
    name        text not null primary key,
    body        blob not null,
    create_time  datetime default current_timestamp,
    last_viewed  datetime default current_timestamp
);
