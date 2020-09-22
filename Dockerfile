FROM golang
WORKDIR /src
ADD . .
RUN go install .
CMD ["/go/bin/team-gates"]
EXPOSE 8080
