import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// В разработке /api проксируется на локальный Go-сервер,
// на проде тот же путь отдаёт Caddy — фронт адрес API не знает.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
});
