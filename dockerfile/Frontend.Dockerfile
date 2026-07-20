## ---- Stage 1: Generate WASM artifacts ----
#FROM golang:1.26-alpine AS wasm-build
#RUN apk add --no-cache make
#RUN apk add --no-cache bash
#RUN apk add --no-cache protobuf protobuf-dev
#
#WORKDIR /app
#
## ---- Codegen Layer ----
#COPY Makefile install_build.sh ./
#COPY contracts contracts/
#COPY backend backend/
#
#RUN ./install_build.sh
#
#RUN make frontend-wasm --always-make
#
#RUN ls -lr
#
## ---- Stage 2: Build the Svelte app ----
#FROM node:20-alpine AS build
#RUN apk add --no-cache make
#
#WORKDIR /app
#
#COPY --from=wasm-build /app/frontend/static/wasm ./frontend/static/wasm
#
## ---- Dependency Layer ----
#COPY frontend/package*.json ./
#
#RUN npm install
#
## ---- Codegen Layer ----
#COPY Makefile .
#COPY contracts contracts/
#COPY frontend frontend/
#
#RUN make frontend-js --always-make
#
## ---- Compile Layer ----
#RUN cd frontend && npm run build
#
## ---- Stage 3: Install production-only dependencies ----
#FROM node:20-alpine AS prod-deps
#
#WORKDIR /app
#
#COPY frontend/package*.json ./
#
#RUN npm install --omit=dev
#
## ---- Stage 4: Runner ----
#FROM node:20-alpine AS runner
#
#WORKDIR /app
#
## SvelteKit's Node adapter output is a standalone Node server
#COPY --from=build /app/build ./build
#COPY --from=prod-deps /app/node_modules ./node_modules
#COPY --from=build /app/package*.json ./
#
#ENV NODE_ENV=production
#ENV HOST=0.0.0.0
#ENV PORT=3000
#
#EXPOSE 3000
#
## Run the actual Node.js server produced by the build
#CMD ["node", "build/index.js"]