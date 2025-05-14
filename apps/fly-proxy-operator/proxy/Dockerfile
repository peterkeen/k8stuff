FROM alpine:latest as builder
WORKDIR /app
COPY . ./

FROM tailscale/tailscale:latest as tailscale

FROM nginx:stable-alpine
RUN apk add --no-cache ca-certificates iptables ip6tables

COPY --from=tailscale /usr/local/bin/tailscaled /usr/local/bin
COPY --from=tailscale /usr/local/bin/tailscale /usr/local/bin
COPY --from=builder /app/proxy.conf /etc/nginx/conf.d/proxy.conf
COPY --from=builder /app/docker-entrypoint.d/* /docker-entrypoint.d
