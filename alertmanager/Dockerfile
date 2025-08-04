FROM alpine:latest AS builder

# Instala o pacote 'gettext', que contém a ferramenta 'envsubst'.
RUN apk add --no-cache gettext

ARG WEBHOOK_URL
COPY alertmanager/alertmanager.yml.template /
RUN export WEBHOOK_URL=${WEBHOOK_URL} && envsubst < /alertmanager.yml.template > /alertmanager.yml

FROM prom/alertmanager:latest

COPY --from=builder /alertmanager.yml /etc/alertmanager/alertmanager.yml