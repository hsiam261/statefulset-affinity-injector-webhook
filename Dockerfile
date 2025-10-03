FROM busybox:latest

COPY build/bin/statefulset-affinity-injector /usr/local/bin/statefulset-affinity-injector

EXPOSE 8080
EXPOSE 8443

CMD /usr/local/bin/statefulset-affinity-injector
