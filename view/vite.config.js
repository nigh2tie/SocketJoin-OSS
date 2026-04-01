import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

const apiOrigin = process.env.VITE_API_URL || 'http://localhost:8080';
const wsOrigin = apiOrigin.replace(/^http/, 'ws');

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': apiOrigin,
			'/ws': {
				target: wsOrigin,
				ws: true
			}
		}
	}
});
