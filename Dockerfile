FROM golang:1.14 as go_build
WORKDIR /build/

COPY main.go .
COPY go.mod .
RUN GOOS=linux CGO_ENABLED=0 go build -a -installsuffix cgo -o sox_aas .

FROM ubuntu

RUN apt update && apt install -y sox libsox-fmt-all

WORKDIR /root/
COPY --from=go_build ["/build/sox_aas", "/root/"]

CMD ["/root/sox_aas"]
