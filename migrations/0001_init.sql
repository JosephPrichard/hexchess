-- +goose up

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';

CREATE TYPE public.cause_enum AS ENUM (
    'CHECKMATE',
    'FORFEIT',
    'STALEMATE'
);

CREATE TYPE public.color_enum AS ENUM (
    'WHITE',
    'BLACK',
    'RANDOM'
);

CREATE TYPE public.mode_enum AS ENUM (
    'TIMED_1+0',
    'TIMED_3+2',
    'TIMED_15+10',
    'CORRESPONDENCE_1',
    'CORRESPONDENCE_7',
    'CORRESPONDENCE_14',
    'TIMED_5+0'
);

CREATE TYPE public.queue_type_enum AS ENUM (
    'TOURNAMENT_ADVANCE_EVENT'
);

CREATE TYPE public.result_enum AS ENUM (
    'WHITE_WINS',
    'BLACK_WINS',
    'DRAW',
    'RANDOM'
);

CREATE TYPE public.tournament_ruleset_enum AS ENUM (
    'KNOCKOUT',
    'ROUND_ROBIN',
    'SWISS'
);

CREATE TYPE public.tournament_status_enum AS ENUM (
    'LOBBY',
    'SCHEDULED',
    'IN_PROGRESS',
    'FINISHED',
    'CANCELLED'
);

CREATE TABLE public.challenges (
    challenger_id bigint NOT NULL,
    challengee_id bigint NOT NULL,
    start_color public.color_enum NOT NULL,
    made_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    mode public.mode_enum NOT NULL
);

CREATE TABLE public.event_keys (
    id uuid NOT NULL,
    data bytea NOT NULL,
    consumed_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE SEQUENCE public.games_metadata_ordering_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE public.games_metadata (
    ordering bigint DEFAULT nextval('public.games_metadata_ordering_seq'::regclass) NOT NULL,
    game_id text NOT NULL,
    mode public.mode_enum NOT NULL,
    white_id bigint,
    black_id bigint,
    updated_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE VIEW public.games_metadata_count AS
SELECT count(*) AS total
FROM public.games_metadata;

CREATE TABLE public.replay_move_histories (
    replay_id bigint NOT NULL,
    data bytea NOT NULL
);

CREATE TABLE public.replays (
    id bigint NOT NULL,
    white_id bigint,
    black_id bigint,
    mode public.mode_enum NOT NULL,
    result public.result_enum NOT NULL,
    cause public.cause_enum NOT NULL,
    played_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    win_elo_diff double precision NOT NULL,
    lose_elo_diff double precision NOT NULL,
    white_elo double precision NOT NULL,
    black_elo double precision NOT NULL,
    game_id text NOT NULL,
    turn_count integer DEFAULT 0 NOT NULL,
    rating double precision GENERATED ALWAYS AS (((white_elo + black_elo) / (2)::double precision)) STORED,
    played_on_as_days integer GENERATED ALWAYS AS (((EXTRACT(epoch FROM ((played_on AT TIME ZONE 'UTC'::text) - '1970-01-01 00:00:00'::timestamp without time zone)) / (86400)::numeric))::integer) STORED
);

ALTER TABLE public.replays ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.replays_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE public.tournament_matches (
    ordering bigint NOT NULL,
    tournament_key uuid NOT NULL,
    round integer NOT NULL,
    game_id text NOT NULL,
    created_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    white_id bigint NOT NULL,
    black_id bigint NOT NULL
);

ALTER TABLE public.tournament_matches ALTER COLUMN ordering ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tournament_matches_ordering_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE public.tournament_participants (
    tournament_key uuid NOT NULL,
    user_id bigint NOT NULL,
    joined_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.tournaments (
    id bigint NOT NULL,
    tournament_key uuid NOT NULL,
    name text NOT NULL,
    rounds integer NOT NULL,
    status public.tournament_status_enum NOT NULL,
    mode public.mode_enum NOT NULL,
    countdown bigint NOT NULL,
    countdown_started_on timestamp with time zone,
    created_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by bigint NOT NULL,
    updated_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ruleset public.tournament_ruleset_enum DEFAULT 'KNOCKOUT'::public.tournament_ruleset_enum NOT NULL,
    winner_id bigint
);

ALTER TABLE public.tournaments ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tournaments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE public.user_mode_elos (
    user_id bigint NOT NULL,
    mode public.mode_enum NOT NULL,
    elo double precision NOT NULL,
    highest_elo double precision NOT NULL,
    wins integer DEFAULT 0 NOT NULL,
    losses integer DEFAULT 0 NOT NULL,
    draws integer DEFAULT 0 NOT NULL
);

CREATE TABLE public.users (
    id bigint NOT NULL,
    username character varying NOT NULL,
    country character varying NOT NULL,
    bio character varying DEFAULT ''::character varying NOT NULL,
    joined_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    password character varying NOT NULL,
    salt character varying NOT NULL,
    login_attempts integer DEFAULT 0 NOT NULL,
    last_login_attempt timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    google_account_id character varying
);

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE public.users_metadata (
    id bigint NOT NULL,
    count integer
);

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_pkey PRIMARY KEY (challenger_id, challengee_id);

ALTER TABLE ONLY public.event_keys
    ADD CONSTRAINT event_keys_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.games_metadata
    ADD CONSTRAINT games_metadata_ordering_key UNIQUE (ordering);

ALTER TABLE ONLY public.games_metadata
    ADD CONSTRAINT games_metadata_pkey PRIMARY KEY (game_id);

ALTER TABLE ONLY public.replay_move_histories
    ADD CONSTRAINT replay_move_histories_pkey PRIMARY KEY (replay_id);

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_pkey PRIMARY KEY (ordering);

ALTER TABLE ONLY public.tournament_participants
    ADD CONSTRAINT tournament_participants_pkey PRIMARY KEY (tournament_key, user_id);

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_tournament_key_key UNIQUE (tournament_key);

ALTER TABLE ONLY public.user_mode_elos
    ADD CONSTRAINT user_mode_elos_pkey PRIMARY KEY (user_id, mode);

ALTER TABLE ONLY public.users_metadata
    ADD CONSTRAINT users_metadata_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

CREATE INDEX idx_black_id ON public.replays USING btree (black_id, id);

CREATE INDEX idx_blackid_sort_id ON public.replays USING btree (black_id, id);

CREATE INDEX idx_blackid_sort_rating ON public.replays USING btree (black_id, rating);

CREATE INDEX idx_blackid_sort_turncount ON public.replays USING btree (black_id, turn_count);

CREATE INDEX idx_both_ids ON public.replays USING btree (white_id, black_id, id);

CREATE INDEX idx_both_ids_played_on ON public.replays USING btree (white_id, black_id, played_on);

CREATE INDEX idx_challengee ON public.challenges USING btree (challengee_id, made_on);

CREATE INDEX idx_challenger ON public.challenges USING btree (challenger_id, made_on);

CREATE UNIQUE INDEX idx_google_account_id ON public.users USING btree (google_account_id);

CREATE INDEX idx_playedon ON public.replays USING btree (played_on_as_days);

CREATE UNIQUE INDEX idx_replay_game_id ON public.replays USING btree (game_id);

CREATE INDEX idx_sort_rating ON public.replays USING btree (rating);

CREATE INDEX idx_sort_turncount ON public.replays USING btree (turn_count);

CREATE INDEX idx_tournament_matches_tournament_key ON public.tournament_matches USING btree (tournament_key);

CREATE INDEX idx_tournament_participants_tournament_key ON public.tournament_participants USING btree (tournament_key);

CREATE INDEX idx_tournament_participants_userid ON public.tournament_participants USING btree (user_id);

CREATE INDEX idx_tournament_winner_id ON public.tournaments USING btree (winner_id);

CREATE INDEX idx_trgm_username ON public.users USING gist (username public.gist_trgm_ops);

CREATE UNIQUE INDEX idx_unique_username ON public.users USING btree (upper((username)::text));

CREATE INDEX idx_user_mode_elos_userid ON public.user_mode_elos USING btree (user_id);

CREATE INDEX idx_username ON public.users USING btree (username);

CREATE INDEX idx_white_id ON public.replays USING btree (white_id, id);

CREATE INDEX idx_whiteid_sort_id ON public.replays USING btree (white_id, id);

CREATE INDEX idx_whiteid_sort_rating ON public.replays USING btree (white_id, rating);

CREATE INDEX idx_whiteid_sort_turncount ON public.replays USING btree (white_id, turn_count);

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_challengee_id_fkey FOREIGN KEY (challengee_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_challenger_id_fkey FOREIGN KEY (challenger_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.replay_move_histories
    ADD CONSTRAINT fk_replay_id FOREIGN KEY (replay_id) REFERENCES public.replays(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_black_id_fkey FOREIGN KEY (black_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_white_id_fkey FOREIGN KEY (white_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_black_id_fkey FOREIGN KEY (black_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_tournament_key_fkey FOREIGN KEY (tournament_key) REFERENCES public.tournaments(tournament_key);

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_white_id_fkey FOREIGN KEY (white_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.tournament_participants
    ADD CONSTRAINT tournament_participants_tournament_key_fkey FOREIGN KEY (tournament_key) REFERENCES public.tournaments(tournament_key);

ALTER TABLE ONLY public.tournament_participants
    ADD CONSTRAINT tournament_participants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_winner_id_fkey FOREIGN KEY (winner_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.user_mode_elos
    ADD CONSTRAINT user_mode_elos_userid_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

CREATE TABLE redis_queue_metrics (
    event_id UUID PRIMARY KEY,
    group_id UUID,
    stream_name TEXT NOT NULL,
    consumed_on TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_on TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_redis_queue_metrics_group_id
    ON redis_queue_metrics (group_id);

-- +goose down
DROP SCHEMA public CASCADE;