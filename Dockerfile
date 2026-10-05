FROM golang:1.25 AS build

WORKDIR /src

# Trust corporate Zscaler CA for TLS-inspected HTTPS
COPY zscaler-root-ca.crt /usr/local/share/ca-certificates/zscaler-root-ca.crt
RUN update-ca-certificates
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server .

FROM gcr.io/distroless/static-debian12

ENV PORT=8080 \
    MONGO_URI=mongodb://mongo:27017

COPY --from=build /server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
