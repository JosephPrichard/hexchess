--
-- PostgreSQL database dump
--


-- Dumped from database version 17.5
-- Dumped by pg_dump version 17.10 (Homebrew)

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
-- Name: redis_queue_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.redis_queue_metrics (
                                            event_id uuid NOT NULL,
                                            group_id uuid,
                                            stream_name text NOT NULL,
                                            consumed_on timestamp with time zone NOT NULL,
                                            processed_on timestamp with time zone NOT NULL
);


--
-- Name: redis_queue_metrics redis_queue_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.redis_queue_metrics
    ADD CONSTRAINT redis_queue_metrics_pkey PRIMARY KEY (event_id);

--
-- Name: idx_redis_queue_metrics_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_redis_queue_metrics_group_id ON public.redis_queue_metrics USING btree (group_id);

--
-- PostgreSQL database dump complete
--