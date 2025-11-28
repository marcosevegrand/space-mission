/// <reference types="vite/client" />

export {};

declare global {
  interface Window {
    ENV?: {
      API_URL?: string;
    };
  }
}
