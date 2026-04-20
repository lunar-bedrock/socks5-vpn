FROM golang:1.25.1-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/socks5-vpn ./

FROM alpine:3.21

RUN apk add --no-cache ca-certificates dumb-init iproute2 iptables openvpn

COPY --from=build /out/socks5-vpn /usr/local/bin/socks5-vpn
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/socks5-vpn /usr/local/bin/entrypoint.sh

EXPOSE 1080/tcp
EXPOSE 1080/udp
EXPOSE 8080/tcp

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/bin/entrypoint.sh"]
