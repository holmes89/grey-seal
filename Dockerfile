FROM golang:1.26 AS deps

WORKDIR /grey-seal

ADD *.mod *.sum ./
ENV GOPRIVATE=github.com/holmes89/*
RUN --mount=type=secret,id=gh_pat \
    git config --global url."https://x-access-token:$(cat /run/secrets/gh_pat)@github.com/".insteadOf "https://github.com/" && \
    go mod download

FROM deps AS build
ADD cmd ./cmd
ADD lib ./lib
ADD main.go ./
ARG GIT_HASH=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.commit=${GIT_HASH} -X main.buildTime=${BUILD_TIME}" -o api cmd/api/*.go

FROM scratch
COPY --from=deps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /grey-seal/api /api
COPY --from=build /grey-seal/lib/repo/migrations /migrations
EXPOSE 9000
CMD ["/api"]
