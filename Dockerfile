# syntax=docker/dockerfile:1
FROM golang:1.17 AS builder
WORKDIR /codepipeline
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cdk-codepipeline-go .

FROM alpine:latest  
ARG AWS_CDK_VERSION=1.133.0
RUN apk -v --no-cache --update add \
        nodejs \
        npm \
        python3 \
        ca-certificates \
        groff \
        less \
        bash \
        make \
        curl \
        wget \
        zip \
        git \
        && \
    update-ca-certificates && \
    npm install -g aws-cdk@${AWS_CDK_VERSION}
WORKDIR /root/
COPY --from=builder /codepipeline/cdk-codepipeline-go .
COPY cdk.json .
ENTRYPOINT ["cdk"]
