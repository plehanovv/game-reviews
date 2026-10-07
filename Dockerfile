FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM scratch
COPY --from=build /api /api
USER 65532:65532
ENV HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/api"]
