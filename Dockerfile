FROM golang:1.25-alpine AS build

WORKDIR /w
COPY ./ .
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /usr/local/bin/server ./server

FROM alpine:3.24

COPY --from=build /usr/local/bin/server /usr/local/bin/server
EXPOSE 14236
VOLUME /data
ENV OOTMM_SERVER_DATA_DIR=/data
CMD ["/usr/local/bin/server"]
