FROM alpine:3.23@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375
RUN adduser -k /dev/null -u 10001 -D helm-schema \
  && chgrp 0 /home/helm-schema \
  && chmod -R g+rwX /home/helm-schema
COPY helm-schema /
USER 10001
VOLUME [ "/home/helm-schema" ]
WORKDIR /home/helm-schema
ENTRYPOINT ["/helm-schema"]
