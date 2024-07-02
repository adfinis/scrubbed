FROM docker.io/library/python:3.12 as scrubbed

WORKDIR /src

COPY Makefile initenv.sh requirements.txt scrubbed.py .

RUN make static

FROM quay.io/vshn/signalilo:v0.14.0 as signalilo

FROM debian
#FROM registry.access.redhat.com/ubi9/ubi-micro:9.4

COPY --from=signalilo /usr/local/bin/signalilo /usr/local/bin/

COPY --from=scrubbed /src/dist/scrubbed /usr/local/bin/

EXPOSE 8080
