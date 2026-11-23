# The tickets service: a static binary on a distroless, non-root base.
FROM golang:1.25.4 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tickets .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /tickets /tickets
ENTRYPOINT ["/tickets"]
