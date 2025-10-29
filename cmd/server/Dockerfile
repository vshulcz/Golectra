ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS build
RUN apk add --no-cache ca-certificates build-base git
WORKDIR /src

# deps cache
COPY go.mod go.sum ./
RUN go mod download

# source
COPY . .

# build server
WORKDIR /src/cmd/server
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/golectra-server .

# minimal image
FROM gcr.io/distroless/static:nonroot
USER nonroot:nonroot
COPY --from=build /out/golectra-server /golectra-server
EXPOSE 8080
ENTRYPOINT ["/golectra-server"]
