--
-- PostgreSQL database dump
--

-- Dumped from database version 17.0
-- Dumped by pg_dump version 17.0

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
-- Name: result_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.result_enum AS ENUM (
    'WHITE_WINS',
    'BLACK_WINS',
    'DRAW',
    'RANDOM'
);


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
-- Name: replays; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.replays (
    id bigint NOT NULL,
    white_id bigint NOT NULL,
    black_id bigint NOT NULL,
    mode public.mode_enum NOT NULL,
    result public.result_enum NOT NULL,
    cause public.cause_enum NOT NULL,
    played_on timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    win_elo_diff double precision NOT NULL,
    lose_elo_diff double precision NOT NULL,
    white_elo double precision NOT NULL,
    black_elo double precision NOT NULL,
    move_history bytea NOT NULL
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
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
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
    losses integer DEFAULT 0 NOT NULL
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
-- Name: challenges challenges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.challenges
    ADD CONSTRAINT challenges_pkey PRIMARY KEY (challenger_id, challengee_id);


--
-- Name: replays replays_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.replays
    ADD CONSTRAINT replays_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


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
-- Name: user_mode_elos user_mode_elos_userid_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_mode_elos
    ADD CONSTRAINT user_mode_elos_userid_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

