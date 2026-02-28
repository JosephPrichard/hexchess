-- +goose up

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

COMMENT ON SCHEMA public IS '';

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


SET default_tablespace = '';

SET default_table_access_method = heap;

CREATE TABLE public.challenges (
    challenger_id bigint NOT NULL,
    challengee_id bigint NOT NULL,
    time_control character varying NOT NULL,
    start_color character varying NOT NULL,
    made_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT start_color_check CHECK (((start_color)::text = ANY ((ARRAY['RANDOM'::character varying, 'BLACK'::character varying, 'WHITE'::character varying])::text[]))),
    CONSTRAINT time_control_check CHECK (((time_control)::text = ANY ((ARRAY['REAL_TIME'::character varying, 'CORRESPONDENCE'::character varying, 'UNLIMITED'::character varying])::text[])))
);

CREATE TABLE public.replays (
    id bigint NOT NULL,
    white_id bigint NOT NULL,
    black_id bigint NOT NULL,
    mode character varying NOT NULL,
    result character varying NOT NULL,
    cause character varying NOT NULL,
    played_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    win_elo_diff double precision NOT NULL,
    lose_elo_diff double precision NOT NULL,
    white_elo double precision NOT NULL,
    black_elo double precision NOT NULL,
    move_history bytea NOT NULL,
    CONSTRAINT cause_check CHECK (((cause)::text = ANY ((ARRAY['CHECKMATE'::character varying, 'FORFEIT'::character varying])::text[]))),
    CONSTRAINT mode_check CHECK (((mode)::text = ANY ((ARRAY['REAL_TIME'::character varying, 'CORRESPONDENCE'::character varying, 'UNLIMITED'::character varying])::text[]))),
    CONSTRAINT result_check CHECK (((result)::text = ANY ((ARRAY['DRAW'::character varying, 'WHITE_WINS'::character varying, 'BLACK_WINS'::character varying])::text[])))
);

ALTER TABLE public.replays ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.replays_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE public.users (
    id bigint NOT NULL,
    username character varying NOT NULL,
    country character varying NOT NULL,
    elo double precision NOT NULL,
    highest_elo double precision NOT NULL,
    wins integer NOT NULL,
    losses integer NOT NULL,
    bio character varying DEFAULT ''::character varying NOT NULL,
    joined_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    start_elo double precision NOT NULL,
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

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.users_metadata
    ADD CONSTRAINT users_metadata_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

CREATE INDEX idx_black_id ON public.replays USING btree (black_id, id);

CREATE INDEX idx_both_ids ON public.replays USING btree (white_id, black_id, id);

CREATE INDEX idx_both_ids_played_on ON public.replays USING btree (white_id, black_id, played_on);

CREATE INDEX idx_challengee ON public.challenges USING btree (challengee_id, made_on);

CREATE INDEX idx_challenger ON public.challenges USING btree (challenger_id, made_on);

CREATE INDEX idx_elo ON public.users USING btree (elo);

CREATE INDEX idx_trgm_username ON public.users USING gist (username public.gist_trgm_ops);

CREATE UNIQUE INDEX idx_unique_username ON public.users USING btree (upper((username)::text));

CREATE INDEX idx_username ON public.users USING btree (username);

CREATE INDEX idx_white_id ON public.replays USING btree (white_id, id);

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_challengee_id_fkey FOREIGN KEY (challengee_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_challenger_id_fkey FOREIGN KEY (challenger_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_black_id_fkey FOREIGN KEY (black_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_white_id_fkey FOREIGN KEY (white_id) REFERENCES public.users(id);

