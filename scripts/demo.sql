drop table if exists orders;
drop table if exists users;

create table users (
    id          serial primary key,
    email       text not null,
    name        text,
    created_at  timestamptz not null default now()
);

create table orders (
    id         serial primary key,
    user_id    int not null references users(id),
    amount     numeric(10, 2) not null,
    status     text not null default 'pending',
    placed_at  timestamptz not null default now()
);

insert into users (email, name) values
    ('ada@example.com', 'Ada Lovelace'),
    ('alan@example.com', 'Alan Turing'),
    ('grace@example.com', 'Grace Hopper'),
    ('linus@example.com', 'Linus Torvalds'),
    ('margaret@example.com', 'Margaret Hamilton'),
    ('dennis@example.com', 'Dennis Ritchie'),
    ('ken@example.com', 'Ken Thompson'),
    ('donald@example.com', 'Donald Knuth'),
    ('barbara@example.com', 'Barbara Liskov'),
    ('james@example.com', 'James Gosling');

insert into orders (user_id, amount, status)
select
    (i % 10) + 1,
    round((random() * 500 + 10)::numeric, 2),
    (array['pending', 'shipped', 'delivered', 'cancelled'])[(i % 4) + 1]
from generate_series(1, 120) as i;