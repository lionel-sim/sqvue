drop table if exists inventory;
drop table if exists reviews;
drop table if exists products;
drop table if exists categories;
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

create table categories (
    id        serial primary key,
    name      text not null,
    parent_id int references categories(id)
);

create table products (
    id          serial primary key,
    category_id int not null references categories(id),
    name        text not null,
    price       numeric(10, 2) not null,
    in_stock    boolean not null default true,
    tags        text[],
    metadata    jsonb,
    released_on date
);

create table reviews (
    id         serial primary key,
    product_id int not null references products(id),
    rating     int not null check (rating between 1 and 5),
    comment    text,
    created_at timestamptz not null default now()
);

create table inventory (
    id            serial primary key,
    sku           text not null,
    name          text not null,
    category      text,
    price         numeric(10, 2) not null,
    cost          numeric(10, 2),
    quantity      int not null,
    reorder_level int not null default 10,
    in_stock      boolean not null default true,
    warehouse     text,
    tags          text[],
    metadata      jsonb,
    updated_at    timestamptz not null default now()
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

insert into categories (name, parent_id) values
    ('Electronics', null),
    ('Computers', 1),
    ('Audio', 1),
    ('Books', null),
    ('Fiction', 4);

insert into products (category_id, name, price, in_stock, tags, metadata, released_on) values
    (2, 'ThinkPad X1', 1499.99, true,  array['laptop', 'business'], '{"warranty": 3, "weight_kg": 1.13}', '2024-03-15'),
    (2, 'MacBook Air M3', 1099.00, false, array['laptop', 'apple'],  '{"warranty": 1}',                 '2024-03-08'),
    (3, 'AirPods Pro 2', 249.00,  true,  array['earbuds', 'noise-cancelling'], '{"color": "white"}',    '2022-09-23'),
    (3, 'Sonos One',     219.00,  true,  array['speaker', 'wifi'],     null,                             null),
    (5, 'Dune',          18.99,   true,  array['scifi', 'hardcover'], null,                             '1965-08-01'),
    (5, 'Neuromancer',   14.50,   false, array['cyberpunk'],          null,                             '1984-07-01'),
    (1, 'Raspberry Pi 5', 79.99,  true,  array['sbc', 'hobby'],       '{"ram": 8}',                     '2023-10-23');

insert into reviews (product_id, rating, comment)
select
    (i % 7) + 1,
    (i % 5) + 1,
    case
        when i % 3 = 0 then null
        else 'sample review ' || i
    end
from generate_series(1, 80) as i;

insert into inventory (sku, name, category, price, cost, quantity, reorder_level, in_stock, warehouse, tags, metadata)
select
    'SKU-' || lpad(i::text, 5, '0'),
    'Warehouse item number ' || i,
    (array['peripherals', 'cables', 'storage', 'power', 'mounts'])[(i % 5) + 1],
    round((random() * 200 + 5)::numeric, 2),
    round((random() * 150 + 3)::numeric, 2),
    (i * 7) % 120,
    (i % 25) + 1,
    (i % 3) <> 0,
    (array['A1', 'B2', 'C3', 'D4'])[(i % 4) + 1],
    array['bulk', 'imported', 'clearance'],
    jsonb_build_object('batch', i % 50, 'pallet', 'P-' || (i % 12), 'fragile', (i % 4) = 0)
from generate_series(1, 200) as i;
