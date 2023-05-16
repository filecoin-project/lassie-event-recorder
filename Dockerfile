FROM golang:1.19-bullseye as build

WORKDIR /go/src/lassie-event-recorder

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/recorder ./cmd/recorder

FROM busybox:1.35.0-uclibc as busybox

FROM gcr.io/distroless/static-debian11
COPY --from=build /go/bin/recorder /usr/bin/
COPY --from=busybox /bin/sh /bin/sh

ENTRYPOINT ["/usr/bin/recorder"]
