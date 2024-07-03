FROM docker.io/library/python:3.12 AS scrubbed

WORKDIR /src

COPY Makefile initenv.sh requirements.txt scrubbed.py .

RUN make static

FROM quay.io/vshn/signalilo:v0.14.0 AS signalilo

FROM docker.io/library/debian:bookworm

COPY --from=signalilo /usr/local/bin/signalilo /usr/local/bin/

COPY --from=scrubbed /src/dist/scrubbed /usr/local/bin/
