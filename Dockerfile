FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
ARG TARGETPLATFORM
RUN adduser -k /dev/null -u 10001 -D helm-schema \
  && chgrp 0 /home/helm-schema \
  && chmod -R g+rwX /home/helm-schema
COPY $TARGETPLATFORM/helm-schema /
USER 10001
VOLUME [ "/home/helm-schema" ]
WORKDIR /home/helm-schema
ENTRYPOINT ["/helm-schema"]
