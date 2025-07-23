export const isDev = process.env.NODE_ENV === 'development';
export const host = isDev ? 'http://127.0.0.1:3001' : '';