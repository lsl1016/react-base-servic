FROM python:3.11-slim

# 中文字体（matplotlib 中文标签用，与基座 meta tool 声明一致）
RUN apt-get update \
    && apt-get install -y --no-install-recommends fonts-wqy-zenhei \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt && rm /tmp/requirements.txt

COPY server.py /app/server.py

ENV PYTHONDONTWRITEBYTECODE=1

# 非 root 运行
RUN useradd --create-home --shell /usr/sbin/nologin sandbox \
    && mkdir -p /app && chown -R sandbox:sandbox /app
USER sandbox

EXPOSE 8190

CMD ["python", "/app/server.py"]
