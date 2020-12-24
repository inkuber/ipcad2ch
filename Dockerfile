FROM golang:1.14.6
COPY . /go/src/github.com/inkuber/ipcad2ch/
WORKDIR /go/src/github.com/inkuber/ipcad2ch/
RUN CGO_ENABLED=0 GOOS=linux make build
CMD ["tail", "-f", "Readme.md"]
