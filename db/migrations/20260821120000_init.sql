-- +goose Up
-- Initial schema per docs/mvp-plan.md, section "Datenmodell (MVP-Umfang)".
-- Identities live in a table of their own on purpose, see docs/adr/0003.

create table players (
    id           uuid        primary key default gen_random_uuid(),
    display_name text        not null,
    ttr          int         not null default 1000,
    created_at   timestamptz not null default now(),

    constraint players_display_name_not_blank check (length(btrim(display_name)) > 0)
);

create unique index players_display_name_key on players (lower(btrim(display_name)));

create table identities (
    provider   text        not null,
    subject    text        not null,
    player_id  uuid        not null references players (id) on delete cascade,
    created_at timestamptz not null default now(),

    primary key (provider, subject)
);

create index identities_player_id_idx on identities (player_id);

create table matches (
    id            uuid        primary key default gen_random_uuid(),
    home_id       uuid        not null references players (id),
    away_id       uuid        not null references players (id),
    best_of       int         not null,
    points_to_win int         not null default 11,
    status        text        not null default 'pending',
    reported_by   uuid        not null references players (id),
    played_at     timestamptz not null default now(),
    confirmed_at  timestamptz,

    constraint matches_players_differ check (home_id <> away_id),
    constraint matches_best_of_valid check (best_of in (3, 5, 7)),
    constraint matches_points_to_win_positive check (points_to_win > 0),
    constraint matches_status_valid check (status in ('pending', 'confirmed', 'disputed')),
    -- confirmed_at is set exactly when the status is 'confirmed'.
    constraint matches_confirmed_at_matches_status check (
        (status = 'confirmed') = (confirmed_at is not null)
    )
);

create index matches_home_id_idx on matches (home_id, played_at desc);
create index matches_away_id_idx on matches (away_id, played_at desc);
create index matches_status_idx on matches (status, played_at desc);

create table match_sets (
    match_id    uuid not null references matches (id) on delete cascade,
    set_no      int  not null,
    home_points int  not null,
    away_points int  not null,

    primary key (match_id, set_no),
    constraint match_sets_set_no_positive check (set_no > 0),
    constraint match_sets_points_not_negative check (home_points >= 0 and away_points >= 0),
    constraint match_sets_no_draw check (home_points <> away_points)
);

-- Mandatory in the MVP: without history a faulty calculation cannot be
-- traced and a rating chart cannot be reconstructed.
create table ttr_history (
    id         uuid        primary key default gen_random_uuid(),
    player_id  uuid        not null references players (id) on delete cascade,
    match_id   uuid        not null references matches (id) on delete cascade,
    ttr_before int         not null,
    ttr_after  int         not null,
    created_at timestamptz not null default now()
);

create index ttr_history_player_idx on ttr_history (player_id, created_at desc);
create unique index ttr_history_player_match_key on ttr_history (player_id, match_id);

-- +goose Down
drop table ttr_history;
drop table match_sets;
drop table matches;
drop table identities;
drop table players;
