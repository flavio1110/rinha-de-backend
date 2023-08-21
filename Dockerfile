FROM alpine:3.15.0
RUN apk add --update curl && rm -rf /var/cache/apk/*
COPY bin/rinha-backend /rinha-backend

EXPOSE 9999
ENTRYPOINT /rinha-backend

