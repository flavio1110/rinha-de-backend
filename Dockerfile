FROM alpine:3.15.0

COPY bin/rinha-backend /rinha-backend

ENV WAIT_VERSION 2.7.2
ADD https://github.com/ufoscout/docker-compose-wait/releases/download/$WAIT_VERSION/wait /wait
RUN chmod +x /wait

EXPOSE 9999
ENTRYPOINT /rinha-backend

