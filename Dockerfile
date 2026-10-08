FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /out/wizard ./cmd/wizard

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/wizard /wizard
ENTRYPOINT ["/wizard"]
