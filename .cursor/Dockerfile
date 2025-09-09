FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# System deps and GUI libs for headless NW.js (wow.export)
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates curl git python3 build-essential \
  xvfb \
  libgbm1 libgl1 libxshmfence1 libglib2.0-0 libatk1.0-0 libcups2 \
  libxcomposite1 libxdamage1 libxfixes3 libpango-1.0-0 libcairo2 \
  libasound2t64 libatspi2.0-0 libxrandr2 libxss1 libgtk-3-0 \
  libgdk-pixbuf2.0-0 libnss3 libnspr4 libdrm2 libxcb1 libx11-6 \
  libxext6 libxrender1 libxinerama1 libxi6 libxtst6 libxkbcommon0 \
  libpangocairo-1.0-0 libx11-xcb1 libxcb-dri3-0 upower dbus libatomic1

# Install Node.js LTS (20.x) for building/running the app
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
  && apt-get update && apt-get install -y --no-install-recommends nodejs \
  && rm -rf /var/lib/apt/lists/*

# Do not COPY project sources; Cursor mounts the workspace at runtime

CMD ["bash"]

