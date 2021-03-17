-- Original schema

create table if not exists Notes (
    Name        text not null primary key,
    Body        blob not null,
    CreateTime  datetime default current_timestamp,
    LastViewed  datetime default current_timestamp
);
