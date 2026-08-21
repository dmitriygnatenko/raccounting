FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY build/app/raccounting /app/raccounting

RUN chmod +x /app/raccounting

EXPOSE 8080

ENTRYPOINT ["/app/raccounting"]