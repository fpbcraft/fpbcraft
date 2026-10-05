export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    cache: 'no-store',
    headers: {
      Accept: 'application/json',
      ...(init?.body ? {'Content-Type': 'application/json'} : {}),
      ...(init?.headers ?? {}),
    },
    ...init,
  });
  if (!response.ok) {
    let message = path + ' returned HTTP ' + response.status;
    try {
      const body = (await response.json()) as {error?: string};
      if (body.error) message = body.error;
    } catch {
      // Keep the status-based fallback.
    }
    throw new Error(message);
  }
  return (await response.json()) as T;
}
