import { http, HttpResponse } from 'msw';

export const API_BASE = 'http://localhost:8080/api/v1';

export function mockGet(path: string, body: unknown, status = 200) {
  return http.get(`${API_BASE}${path}`, () => {
    return HttpResponse.json({ success: true, message: 'ok', data: body }, { status });
  });
}

export function mockPost(path: string, body: unknown, status = 201) {
  return http.post(`${API_BASE}${path}`, () => {
    return HttpResponse.json({ success: true, message: 'created', data: body }, { status });
  });
}

export function mockPostHandler(
  path: string,
  handler: (args: { request: Request }) => { status?: number; body: unknown },
) {
  return http.post(`${API_BASE}${path}`, ({ request }) => {
    const result = handler({ request });
    return HttpResponse.json(
      { success: true, message: 'ok', data: result.body },
      { status: result.status ?? 200 },
    );
  });
}

export function mockErrorHandler(path: string, status: number, code: string, message: string) {
  return http.get(`${API_BASE}${path}`, () => {
    return HttpResponse.json(
      { success: false, error: { code, message } },
      { status },
    );
  });
}
