FROM alpine:3.19

RUN apk add --no-cache bash coreutils

WORKDIR /app
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Create outputs directory
RUN mkdir -p /outputs

ENTRYPOINT ["/app/entrypoint.sh"]
