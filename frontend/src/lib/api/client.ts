// Frontend API calls should go through same-origin Next.js routes so that
// httpOnly session cookies are sent consistently and rewrites can proxy to backend.
const API_BASE_URL = '/api/v1';

export { API_BASE_URL };
