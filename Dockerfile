FROM busybox:latest

COPY build/bin/statefulset-affinity-webhook /usr/local/bin/statefulset-affinity-webhook

EXPOSE 8080
EXPOSE 8443

CMD /usr/local/bin/statefulset-affinity-webhook
