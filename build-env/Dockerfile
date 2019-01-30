FROM golang:latest

ENV PROJECT github.com/TykTechnologies/go-plugins-template

COPY ./build.sh ./build.sh

ENTRYPOINT ["/bin/bash", "./build.sh"]
