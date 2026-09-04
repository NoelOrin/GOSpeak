FROM node:20-alpine AS builder

WORKDIR /app
COPY packages/mediasoup-worker/package.json ./
RUN npm install
COPY packages/mediasoup-worker/ ./
RUN npm run build

FROM node:20-alpine

WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules

EXPOSE 3012 40000-49999/udp
CMD ["node", "dist/index.js"]
