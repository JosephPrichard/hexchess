# ---- Stage 1: Build the Svelte app ----
FROM node:20-alpine AS build
RUN apk add --no-cache make

WORKDIR /app

# ---- Dependency Layer ----
COPY frontend/package*.json frontend/

RUN cd frontend && npm install

# ---- Codegen Layer ----
COPY Makefile .
COPY contracts contracts/
COPY frontend frontend/

RUN make frontend --always-make

# ---- Compile Layer ----
RUN cd frontend && npm run build

# ---- Stage 2: Runner ----
FROM node:20-alpine AS runner

WORKDIR /app

# SvelteKit's Node adapter output is a standalone Node server
COPY --from=build /app/frontend/build ./build
# This contains dev dependencies, consider stripping unncessary dependencies
COPY --from=build /app/frontend/node_modules ./node_modules
COPY --from=build /app/frontend/package*.json ./

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=5173

EXPOSE 5173

# Run the actual Node.js server produced by the build
CMD ["node", "build/index.js"]