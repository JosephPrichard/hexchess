import createClient from "openapi-fetch";
import type { components, paths } from './api-v1.d.ts';

export const appBaseURL = 'http://localhost:5173';
export const baseURL = 'http://localhost:8081';

export const client = createClient<paths>({ baseUrl: baseURL });

export type ChallengeView = components["schemas"]["ChallengeView"];
export type UserView = components["schemas"]["UserView"];
export type ReplayView = components["schemas"]["ReplayView"];