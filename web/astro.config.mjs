import { defineConfig } from 'astro/config';

// 정적 사이트로 빌드하고 preview 서버를 3000 포트로 노출한다.
export default defineConfig({
  server: {
    host: true,
    port: 3000,
  },
});
