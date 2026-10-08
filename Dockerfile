FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/outbox ./cmd/outbox \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/workflow ./cmd/workflow \
 && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/seed-slots ./cmd/seed-slots

FROM alpine:3.22
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/outbox /usr/local/bin/outbox
COPY --from=build /out/workflow /usr/local/bin/workflow
COPY --from=build /out/seed-slots /usr/local/bin/seed-slots
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["api"]
