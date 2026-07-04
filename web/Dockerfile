FROM node:20-alpine

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install

COPY . .
RUN npm run build

EXPOSE 3000
# 정적 빌드 결과를 0.0.0.0:3000 으로 서빙한다.
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", "3000"]
