-- +goose Up
create table if not exists white_list (
	cidr cidr not null primary key

);

create table if not exists black_list (
	cidr cidr not null primary key

);

-- +goose Down
drop table if exists white_list;
drop table if exists black_list;
