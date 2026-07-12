FROM golang:1.25 AS build

WORKDIR /src

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
