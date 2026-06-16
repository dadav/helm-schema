FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
ARG TARGETPLATFORM
RUN adduser -k /dev/null -u 10001 -D helm-schema \
  && chgrp 0 /home/helm-schema \
  && chmod -R g+rwX /home/helm-schema
COPY $TARGETPLATFORM/helm-schema /
USER 10001
VOLUME [ "/home/helm-schema" ]
WORKDIR /home/helm-schema
ENTRYPOINT ["/helm-schema"]
