ARG MOSQUITTO_VERSION
FROM eclipse-mosquitto:${MOSQUITTO_VERSION}

COPY ./config/mosquitto.conf /mosquitto/config/
COPY ./config/certs /mosquitto/config/certs/
COPY docker-entrypoint.sh /

ENTRYPOINT ["sh", "./docker-entrypoint.sh"]

CMD ["/usr/sbin/mosquitto", "-v", "-c", "/mosquitto/config/mosquitto.conf"]