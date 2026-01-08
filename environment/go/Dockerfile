FROM bennavetta/jira-base:latest

COPY --from=golang:1.25 /usr/local/go/ /usr/local/go/

ENV GOPATH=/go
ENV PATH=${GOPATH}/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"
WORKDIR $GOPATH