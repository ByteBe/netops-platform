# NetOps 容器镜像
FROM ubuntu:22.04

RUN apt update && apt install -y --no-install-recommends \
    ca-certificates fontconfig fonts-wqy-zenhei fonts-noto-cjk tzdata \
    wkhtmltopdf xvfb \
    && rm -rf /var/lib/apt/lists/* \
    && fc-cache -fv

WORKDIR /app
COPY netops-server /app/netops-server
COPY web/ /app/web/

EXPOSE 30821
CMD ["/app/netops-server"]
