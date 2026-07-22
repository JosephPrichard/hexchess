--
-- PostgreSQL database dump
--


-- Dumped from database version 18.4 (Ubuntu 18.4-0ubuntu0.26.04.1)
-- Dumped by pg_dump version 18.4 (Ubuntu 18.4-0ubuntu0.26.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', 'public', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

-- *not* creating schema, since initdb creates it


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS '';


--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: cause_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.cause_enum AS ENUM (
    'CHECKMATE',
    'FORFEIT',
    'STALEMATE'
);


--
-- Name: color_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.color_enum AS ENUM (
    'WHITE',
    'BLACK',
    'RANDOM'
);


--
-- Name: mode_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.mode_enum AS ENUM (
    'TIMED_1+0',
    'TIMED_3+2',
    'TIMED_15+10',
    'CORRESPONDENCE_1',
    'CORRESPONDENCE_7',
    'CORRESPONDENCE_14',
    'TIMED_5+0'
);


--
-- Name: queue_type_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.queue_type_enum AS ENUM (
    'TOURNAMENT_ADVANCE_EVENT'
);


--
-- Name: result_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.result_enum AS ENUM (
    'WHITE_WINS',
    'BLACK_WINS',
    'DRAW',
    'RANDOM'
);


--
-- Name: river_job_state; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.river_job_state AS ENUM (
    'available',
    'cancelled',
    'completed',
    'discarded',
    'pending',
    'retryable',
    'running',
    'scheduled'
);


--
-- Name: tournament_ruleset_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.tournament_ruleset_enum AS ENUM (
    'KNOCKOUT',
    'ROUND_ROBIN',
    'SWISS'
);


--
-- Name: tournament_status_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.tournament_status_enum AS ENUM (
    'LOBBY',
    'SCHEDULED',
    'IN_PROGRESS',
    'FINISHED',
    'CANCELLED'
);


--
-- Name: river_job_state_in_bitmask(bit, public.river_job_state); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.river_job_state_in_bitmask(bitmask bit, state public.river_job_state) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE state
        WHEN 'available' THEN get_bit(bitmask, 7)
        WHEN 'cancelled' THEN get_bit(bitmask, 6)
        WHEN 'completed' THEN get_bit(bitmask, 5)
        WHEN 'discarded' THEN get_bit(bitmask, 4)
        WHEN 'pending'   THEN get_bit(bitmask, 3)
        WHEN 'retryable' THEN get_bit(bitmask, 2)
        WHEN 'running'   THEN get_bit(bitmask, 1)
        WHEN 'scheduled' THEN get_bit(bitmask, 0)
        ELSE 0
    END = 1;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: challenges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.challenges (
    challenger_id bigint NOT NULL,
    challengee_id bigint NOT NULL,
    start_color public.color_enum NOT NULL,
    made_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    mode public.mode_enum NOT NULL
);


--
-- Name: event_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.event_keys (
    id uuid NOT NULL,
    data bytea NOT NULL,
    consumed_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: games_metadata_ordering_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.games_metadata_ordering_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: games_metadata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.games_metadata (
    ordering bigint DEFAULT nextval('public.games_metadata_ordering_seq'::regclass) NOT NULL,
    game_id text NOT NULL,
    mode public.mode_enum NOT NULL,
    white_id bigint,
    black_id bigint,
    updated_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: games_metadata_count; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.games_metadata_count AS
 SELECT count(*) AS total
   FROM public.games_metadata;

--
-- Name: replay_move_histories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.replay_move_histories (
    replay_id bigint NOT NULL,
    data bytea NOT NULL
);


--
-- Name: replays; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: replays_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.replays ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.replays_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: river_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_job (
    id bigint NOT NULL,
    state public.river_job_state DEFAULT 'available'::public.river_job_state NOT NULL,
    attempt smallint DEFAULT 0 NOT NULL,
    max_attempts smallint DEFAULT 25 NOT NULL,
    attempted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    scheduled_at timestamp with time zone DEFAULT now() NOT NULL,
    priority smallint DEFAULT 1 NOT NULL,
    args jsonb NOT NULL,
    attempted_by text[],
    errors jsonb[],
    kind text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    queue text DEFAULT 'default'::text NOT NULL,
    tags character varying(255)[] DEFAULT '{}'::character varying[] NOT NULL,
    unique_key bytea,
    unique_states bit(8),
    CONSTRAINT finalized_or_finalized_at_null CHECK ((((finalized_at IS NULL) AND (state <> ALL (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))) OR ((finalized_at IS NOT NULL) AND (state = ANY (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))))),
    CONSTRAINT kind_length CHECK (((char_length(kind) > 0) AND (char_length(kind) < 128))),
    CONSTRAINT max_attempts_is_positive CHECK ((max_attempts > 0)),
    CONSTRAINT priority_in_range CHECK (((priority >= 1) AND (priority <= 4))),
    CONSTRAINT queue_length CHECK (((char_length(queue) > 0) AND (char_length(queue) < 128)))
);


--
-- Name: river_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_job_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_job_id_seq OWNED BY public.river_job.id;


--
-- Name: river_leader; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_leader (
    elected_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    leader_id text NOT NULL,
    name text DEFAULT 'default'::text NOT NULL,
    CONSTRAINT leader_id_length CHECK (((char_length(leader_id) > 0) AND (char_length(leader_id) < 128))),
    CONSTRAINT name_length CHECK ((name = 'default'::text))
);


--
-- Name: river_migration; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_migration (
    line text NOT NULL,
    version bigint CONSTRAINT river_migration_version_not_null1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT river_migration_created_at_not_null1 NOT NULL,
    CONSTRAINT line_length CHECK (((char_length(line) > 0) AND (char_length(line) < 128))),
    CONSTRAINT version_gte_1 CHECK ((version >= 1))
);


--
-- Name: river_notification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_notification (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    payload text NOT NULL,
    topic text NOT NULL,
    CONSTRAINT topic_length CHECK (((length(topic) > 0) AND (length(topic) < 128)))
);


--
-- Name: river_notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_notification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_notification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_notification_id_seq OWNED BY public.river_notification.id;


--
-- Name: river_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_queue (
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: tournament_matches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tournament_matches (
    ordering bigint NOT NULL,
    tournament_key uuid NOT NULL,
    round integer NOT NULL,
    game_id text NOT NULL,
    created_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    white_id bigint NOT NULL,
    black_id bigint NOT NULL
);


--
-- Name: tournament_matches_ordering_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tournament_matches ALTER COLUMN ordering ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tournament_matches_ordering_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: tournament_participants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tournament_participants (
    tournament_key uuid NOT NULL,
    user_id bigint NOT NULL,
    joined_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: tournaments; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: tournaments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tournaments ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.tournaments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_mode_elos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_mode_elos (
    user_id bigint NOT NULL,
    mode public.mode_enum NOT NULL,
    elo double precision NOT NULL,
    highest_elo double precision NOT NULL,
    wins integer DEFAULT 0 NOT NULL,
    losses integer DEFAULT 0 NOT NULL,
    draws integer DEFAULT 0 NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: users_metadata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users_metadata (
    id bigint NOT NULL,
    count integer
);


--
-- Name: river_job id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job ALTER COLUMN id SET DEFAULT nextval('public.river_job_id_seq'::regclass);


--
-- Name: river_notification id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_notification ALTER COLUMN id SET DEFAULT nextval('public.river_notification_id_seq'::regclass);


--
-- Name: challenges challenges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_pkey PRIMARY KEY (challenger_id, challengee_id);


--
-- Name: event_keys event_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.event_keys
    ADD CONSTRAINT event_keys_pkey PRIMARY KEY (id);


--
-- Name: games_metadata games_metadata_ordering_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.games_metadata
    ADD CONSTRAINT games_metadata_ordering_key UNIQUE (ordering);


--
-- Name: games_metadata games_metadata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.games_metadata
    ADD CONSTRAINT games_metadata_pkey PRIMARY KEY (game_id);

--
-- Name: replay_move_histories replay_move_histories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.replay_move_histories
    ADD CONSTRAINT replay_move_histories_pkey PRIMARY KEY (replay_id);


--
-- Name: replays replays_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_pkey PRIMARY KEY (id);


--
-- Name: river_job river_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job
    ADD CONSTRAINT river_job_pkey PRIMARY KEY (id);


--
-- Name: river_leader river_leader_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_leader
    ADD CONSTRAINT river_leader_pkey PRIMARY KEY (name);


--
-- Name: river_migration river_migration_pkey1; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_migration
    ADD CONSTRAINT river_migration_pkey1 PRIMARY KEY (line, version);


--
-- Name: river_notification river_notification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_notification
    ADD CONSTRAINT river_notification_pkey PRIMARY KEY (id);


--
-- Name: river_queue river_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_queue
    ADD CONSTRAINT river_queue_pkey PRIMARY KEY (name);


--
-- Name: tournament_matches tournament_matches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_pkey PRIMARY KEY (ordering);


--
-- Name: tournament_participants tournament_participants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_participants
    ADD CONSTRAINT tournament_participants_pkey PRIMARY KEY (tournament_key, user_id);


--
-- Name: tournaments tournaments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_pkey PRIMARY KEY (id);


--
-- Name: tournaments tournaments_tournament_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_tournament_key_key UNIQUE (tournament_key);


--
-- Name: user_mode_elos user_mode_elos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_mode_elos
    ADD CONSTRAINT user_mode_elos_pkey PRIMARY KEY (user_id, mode);


--
-- Name: users_metadata users_metadata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users_metadata
    ADD CONSTRAINT users_metadata_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_black_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_black_id ON public.replays USING btree (black_id, id);


--
-- Name: idx_blackid_sort_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_blackid_sort_id ON public.replays USING btree (black_id, id);


--
-- Name: idx_blackid_sort_rating; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_blackid_sort_rating ON public.replays USING btree (black_id, rating);


--
-- Name: idx_blackid_sort_turncount; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_blackid_sort_turncount ON public.replays USING btree (black_id, turn_count);


--
-- Name: idx_both_ids; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_both_ids ON public.replays USING btree (white_id, black_id, id);


--
-- Name: idx_both_ids_played_on; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_both_ids_played_on ON public.replays USING btree (white_id, black_id, played_on);


--
-- Name: idx_challengee; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_challengee ON public.challenges USING btree (challengee_id, made_on);


--
-- Name: idx_challenger; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_challenger ON public.challenges USING btree (challenger_id, made_on);


--
-- Name: idx_google_account_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_google_account_id ON public.users USING btree (google_account_id);


--
-- Name: idx_playedon; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_playedon ON public.replays USING btree (played_on_as_days);


--
-- Name: idx_replay_game_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_replay_game_id ON public.replays USING btree (game_id);


--
-- Name: idx_sort_rating; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sort_rating ON public.replays USING btree (rating);


--
-- Name: idx_sort_turncount; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sort_turncount ON public.replays USING btree (turn_count);


--
-- Name: idx_tournament_matches_tournament_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tournament_matches_tournament_key ON public.tournament_matches USING btree (tournament_key);


--
-- Name: idx_tournament_participants_tournament_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tournament_participants_tournament_key ON public.tournament_participants USING btree (tournament_key);


--
-- Name: idx_tournament_participants_userid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tournament_participants_userid ON public.tournament_participants USING btree (user_id);


--
-- Name: idx_tournament_winner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tournament_winner_id ON public.tournaments USING btree (winner_id);


--
-- Name: idx_trgm_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_trgm_username ON public.users USING gist (username public.gist_trgm_ops);


--
-- Name: idx_unique_username; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_unique_username ON public.users USING btree (upper((username)::text));


--
-- Name: idx_user_mode_elos_userid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_mode_elos_userid ON public.user_mode_elos USING btree (user_id);


--
-- Name: idx_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_username ON public.users USING btree (username);


--
-- Name: idx_white_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_white_id ON public.replays USING btree (white_id, id);


--
-- Name: idx_whiteid_sort_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_whiteid_sort_id ON public.replays USING btree (white_id, id);


--
-- Name: idx_whiteid_sort_rating; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_whiteid_sort_rating ON public.replays USING btree (white_id, rating);


--
-- Name: idx_whiteid_sort_turncount; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_whiteid_sort_turncount ON public.replays USING btree (white_id, turn_count);


--
-- Name: river_job_args_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_args_index ON public.river_job USING gin (args);


--
-- Name: river_job_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_kind ON public.river_job USING btree (kind);


--
-- Name: river_job_metadata_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_metadata_index ON public.river_job USING gin (metadata);


--
-- Name: river_job_prioritized_fetching_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_prioritized_fetching_index ON public.river_job USING btree (state, queue, priority, scheduled_at, id);


--
-- Name: river_job_state_and_finalized_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_state_and_finalized_at_index ON public.river_job USING btree (state, finalized_at) WHERE (finalized_at IS NOT NULL);


--
-- Name: river_job_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX river_job_unique_idx ON public.river_job USING btree (unique_key) WHERE ((unique_key IS NOT NULL) AND (unique_states IS NOT NULL) AND public.river_job_state_in_bitmask(unique_states, state));


--
-- Name: river_notification_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_notification_created_at_idx ON public.river_notification USING btree (created_at);


--
-- Name: river_notification_topic_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_notification_topic_id_idx ON public.river_notification USING btree (topic, id);


--
-- Name: challenges challenges_challengee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_challengee_id_fkey FOREIGN KEY (challengee_id) REFERENCES public.users(id);


--
-- Name: challenges challenges_challenger_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_challenger_id_fkey FOREIGN KEY (challenger_id) REFERENCES public.users(id);


--
-- Name: replay_move_histories fk_replay_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.replay_move_histories
    ADD CONSTRAINT fk_replay_id FOREIGN KEY (replay_id) REFERENCES public.replays(id) ON DELETE CASCADE;


--
-- Name: replays replays_black_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_black_id_fkey FOREIGN KEY (black_id) REFERENCES public.users(id);


--
-- Name: replays replays_white_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_white_id_fkey FOREIGN KEY (white_id) REFERENCES public.users(id);


--
-- Name: tournament_matches tournament_matches_black_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_black_id_fkey FOREIGN KEY (black_id) REFERENCES public.users(id);


--
-- Name: tournament_matches tournament_matches_tournament_key_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_tournament_key_fkey FOREIGN KEY (tournament_key) REFERENCES public.tournaments(tournament_key);


--
-- Name: tournament_matches tournament_matches_white_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_matches
    ADD CONSTRAINT tournament_matches_white_id_fkey FOREIGN KEY (white_id) REFERENCES public.users(id);


--
-- Name: tournament_participants tournament_participants_tournament_key_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_participants
    ADD CONSTRAINT tournament_participants_tournament_key_fkey FOREIGN KEY (tournament_key) REFERENCES public.tournaments(tournament_key);


--
-- Name: tournament_participants tournament_participants_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournament_participants
    ADD CONSTRAINT tournament_participants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: tournaments tournaments_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: tournaments tournaments_winner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tournaments
    ADD CONSTRAINT tournaments_winner_id_fkey FOREIGN KEY (winner_id) REFERENCES public.users(id);


--
-- Name: user_mode_elos user_mode_elos_userid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_mode_elos
    ADD CONSTRAINT user_mode_elos_userid_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


