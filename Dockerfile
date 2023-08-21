FROM alpine:3.15.0

COPY bin/rinha-backend /rinha-backend

EXPOSE 9999
ENTRYPOINT /rinha-backend

